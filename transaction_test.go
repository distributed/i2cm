// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package i2cm

import (
	"fmt"
	"testing"
)

type alwaysNACK struct{}

func (a *alwaysNACK) Start() error {
	return nil
}

func (a *alwaysNACK) Stop() error {
	return nil
}

func (a *alwaysNACK) ReadByte(ack bool) (byte, error) {
	return 0, nil
}

func (a *alwaysNACK) WriteByte(b byte) error {
	return NACKReceived
}

func TestNoDevice(t *testing.T) {
	var m I2CMaster = &alwaysNACK{}

	tr := NewTransact8x8(m)

	if _, _, err := tr.Transact8x8(Addr7(0), 0, nil, nil); err != NoSuchDevice {
		t.Fatalf("Transact8x8: expected NoSuchDevice, got %T: %#v", err, err)
	}
}

const (
	t_START = iota
	t_STOP
	t_READ
	t_WRITE
)

type i2cItem struct {
	typ int
	b   byte
	ack bool
	err error
}

func (i i2cItem) String() string {
	switch i.typ {
	case t_START:
		return fmt.Sprintf("START > %T: %#v", i.err, i.err)
	case t_STOP:
		return fmt.Sprintf("STOP > %T: %#v", i.err, i.err)
	case t_READ:
		return fmt.Sprintf("READ > %#02x ack %v @ %T: %#v", i.b, i.ack, i.err, i.err)
	case t_WRITE:
		return fmt.Sprintf("WRITE %#02x > %T: %#v", i.b, i.err, i.err)
	}

	return "unknown i2cItem typ"
}

type i2cRecorder struct {
	m   I2CMaster
	log []i2cItem
}

func (r *i2cRecorder) Start() error {
	err := r.m.Start()
	r.log = append(r.log, i2cItem{t_START, 0, false, err})
	return err
}

func (r *i2cRecorder) Stop() error {
	err := r.m.Start()
	r.log = append(r.log, i2cItem{t_STOP, 0, false, err})
	return err
}

func (r *i2cRecorder) ReadByte(ack bool) (byte, error) {
	b, err := r.m.ReadByte(ack)
	r.log = append(r.log, i2cItem{t_READ, b, ack, err})
	return b, err
}

func (r *i2cRecorder) WriteByte(b byte) error {
	err := r.m.WriteByte(b)
	r.log = append(r.log, i2cItem{t_WRITE, b, false, err})
	return err
}

const (
	md8x8_idle = iota
	md8x8_start_received
	md8x8_ignoring
	md8x8_addressed
	md8x8_receive_regaddr
)

const (
	dir_read  = 1
	dir_write = 0
)

type memdev256 struct {
	addr    Addr7
	open    bool
	regaddr uint8
	state   int
	dir     int
	mem     [256]uint8
	lastack bool
}

func newmemdev256(addr Addr7) *memdev256 {
	return &memdev256{addr: addr}
}

func (m *memdev256) Start() error {
	m.state = md8x8_start_received
	return nil
}

// TODO: move these sanity checks into a separate bus sanity checker
func (m *memdev256) Stop() error {
	if m.state == md8x8_idle {
		panic("should not send a Stop bit on idle bus")
	}
	if m.state == md8x8_start_received {
		panic("should not send a stop bit right after a start bit")
	}

	if m.state == md8x8_addressed && m.dir == dir_read {
		if m.lastack == true {
			panic("calling stop on mdev256 in read mode even though the last byte was read with an ACK")
		}
	}

	m.state = md8x8_idle
	return nil
}

func (m *memdev256) WriteByte(b byte) error {
	if m.state == md8x8_start_received {
		if uint8(m.addr.GetBaseAddr()) == b>>1 {
			if b&0x01 != 0 {
				m.state = md8x8_addressed
				m.dir = dir_read
				return nil
			}

			m.state = md8x8_receive_regaddr
			m.dir = dir_write
			return nil
		}

		return NACKReceived

	} else if m.state == md8x8_receive_regaddr {
		m.regaddr = b
		m.state = md8x8_addressed
		return nil

	} else if m.state == md8x8_addressed {
		m.mem[m.regaddr] = b
		m.regaddr++
		return nil
	}

	panic(fmt.Sprintf("not legal to call WriteByte on memdev256 in state %d\n", m.state))
}

func (m *memdev256) ReadByte(ack bool) (byte, error) {
	if m.state == md8x8_addressed {
		if m.dir == dir_read {
			b := m.mem[m.regaddr]
			m.regaddr++
			m.lastack = ack
			return b, nil
		}

		panic("it is illegal to read from an mdev256 in write mode")
	}

	panic("memdev256 cannot be read from when not addressed")
}

func TestTransactionLog(t *testing.T) {
	// currently there are only non-failing use cases of Transact8x8
	// in this function. a failing use case of Transact8x8 can be
	// found in TestNoDevice
	cases := []struct {
		regaddr uint8
		wb      []byte // bytes to write
		erb     []byte // bytes expected to be written
		explog  []i2cItem
	}{{0x34, []byte{0xfe}, nil, []i2cItem{{t_START, 0, false, nil}, // random write
		{t_WRITE, 0xa0, false, nil},
		{t_WRITE, 0x34, false, nil},
		{t_WRITE, 0xfe, false, nil},
		{t_STOP, 0x00, false, nil},
	}},
		{0x50, nil, nil, []i2cItem{{t_START, 0, false, nil}, // just addr write
			{t_WRITE, 0xa0, false, nil},
			{t_WRITE, 0x50, false, nil},
			{t_STOP, 0x00, false, nil},
		}},
		{0x30, nil, []byte{0x80, 0x81}, []i2cItem{{t_START, 0, false, nil}, // addr write, then read
			{t_WRITE, 0xa0, false, nil},
			{t_WRITE, 0x30, false, nil},
			{t_START, 0x00, false, nil},
			{t_WRITE, 0xa1, false, nil},
			{t_READ, 0x80, true, nil},
			{t_READ, 0x81, false, nil},
			{t_STOP, 0x00, false, nil},
		}},
		{0x22, []byte{0xab, 0xcd}, []byte{0x01, 0x02, 0x03}, []i2cItem{{t_START, 0, false, nil}, // addr write, data write, then read
			{t_WRITE, 0xa0, false, nil},
			{t_WRITE, 0x22, false, nil},
			{t_WRITE, 0xab, false, nil},
			{t_WRITE, 0xcd, false, nil},
			{t_START, 0x00, false, nil},
			{t_WRITE, 0xa1, false, nil},
			{t_READ, 0x01, true, nil},
			{t_READ, 0x02, true, nil},
			{t_READ, 0x03, false, nil},
			{t_STOP, 0x00, false, nil},
		}}}

caseloop:
	for j, tc := range cases {
		md256 := newmemdev256(Addr7(0xa0 >> 1))
		m := &i2cRecorder{md256, nil}

		so := int(tc.regaddr) + len(tc.wb)
		eo := so + len(tc.erb)
		copy(md256.mem[so:eo], tc.erb)

		rb := make([]byte, len(tc.erb))

		nw, nr, err := NewTransact8x8(m).Transact8x8(Addr7(0xa0>>1), tc.regaddr, tc.wb, rb)

		if err != nil {
			t.Errorf("transaction %d is not expected to fail. it returned the error %T: %#v\n", j, err, err)
			continue caseloop
		}

		if nw != len(tc.wb) {
			t.Errorf("transaction %d: expected %d bytes written, got %d\n", j, len(tc.wb), nw)
			continue
		}

		if nr != len(tc.erb) {
			t.Errorf("transaction %d: expected %d bytes read, got %d\n", j, len(tc.erb), nr)
			continue
		}

		if len(m.log) > len(tc.explog) {
			t.Errorf("real log for test case %d is longer than expected log\n", j)
			t.Errorf("real log: %#v\n", m.log)
			t.Errorf("exp log: %#v\n", tc.explog)
			continue caseloop
		}

		// checking what the transaction has read vs. what it should have read
		if string(rb) != string(tc.erb) {
			t.Errorf("test case %d: expected transaction to read % x, it read % x\n", j, tc.erb, rb)
			continue caseloop
		}

		// checking what the transaction wrote into the memdev256's memory
		byteswritten := md256.mem[tc.regaddr : int(tc.regaddr)+len(tc.wb)]
		if string(tc.wb) != string(byteswritten) {
			t.Errorf("test case %d: expected the transaction to have written % x, it did write % x\n", j, tc.wb, byteswritten)
		}

		// check i2c log
		for i, e := range m.log {
			if e != tc.explog[i] {
				t.Errorf("test case %d: i2c log differs at item %d. expected %v, got %v", j, i, tc.explog[i], e)
				continue caseloop
			}
		}
	}
}
