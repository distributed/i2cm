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
	var md256 = newmemdev256(Addr7(0xa0 >> 1))
	var m = &i2cRecorder{md256, nil}

	NewTransact8x8(m).Transact8x8(Addr7(0xa0>>1), 34, []byte{0xab, 0xcd}, []byte{0x01, 0x02, 0x03})

	for _, e := range m.log {
		t.Logf("e %v", e)
	}
}
