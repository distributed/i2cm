// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package i2cm

import (
	"io"
	"testing"
)

// log entry for something x 8 bit transfers
type tXx8item struct {
	addr    Addr
	regaddr uint16
	wb, rb  []byte
	nw, nr  int
	err     error
}

// page verifying transactor for 24Cxx style EEPROMs
// it's always based at address (0xA0 >> 1).
// rollover inside a page is not supported as this
// behavior is not exploited by the EEPROM drivers
// in this package.
// also logs.
type PVT24 struct {
	t        *testing.T
	mem      []byte
	pagesize uint
	log      []tXx8item
}

func (p *PVT24) rwhandler(memaddr uint, startpagebase uint, wb, rb []byte) {
	for _, b := range wb {
		newpagebase := memaddr & ^(p.pagesize - 1)
		if newpagebase != startpagebase {
			p.t.Errorf("EEPROM transaction started in page %#04x, continue to page %#04x", startpagebase, newpagebase)
		}

		waddr := (memaddr & (p.pagesize - 1)) | startpagebase

		p.mem[waddr] = b

		memaddr++
	}

	for i := range rb {
		rb[i] = p.mem[memaddr]
		memaddr++
	}
}

func (p *PVT24) Transact8x8(addr Addr, regaddr uint8, wb, rb []byte) (int, int, error) {
	// read and write logic is intentionally kept simple and different
	// in style from the eeprom routines. maybe i will make different 
	// mistakes both way around :)

	if len(wb) > 0 && len(rb) > 0 {
		p.t.Errorf("trying to write to and read from an EEPROM. this is not allowed.\n")
	}

	memaddr := ((uint(addr.GetBaseAddr()) & 0x07) << 8) + uint(regaddr)
	startpagebase := memaddr & ^(p.pagesize - 1)

	p.rwhandler(memaddr, startpagebase, wb, rb)

	p.log = append(p.log, tXx8item{addr, uint16(regaddr), wb, rb, len(wb), len(rb), nil})

	return len(wb), len(rb), nil
}

func (p *PVT24) Transact16x8(addr Addr, regaddr uint16, wb, rb []byte) (int, int, error) {
	// read and write logic is intentionally kept simple and different
	// in style from the eeprom routines. maybe i will make different 
	// mistakes both way around :)

	if len(wb) > 0 && len(rb) > 0 {
		p.t.Errorf("trying to write to and read from an EEPROM. this is not allowed.\n")
	}

	memaddr := ((uint(addr.GetBaseAddr()) & 0x07) << 16) + uint(regaddr)
	startpagebase := memaddr & ^(p.pagesize - 1)

	p.rwhandler(memaddr, startpagebase, wb, rb)

	p.log = append(p.log, tXx8item{addr, regaddr, wb, rb, len(wb), len(rb), nil})

	return len(wb), len(rb), nil
}

// has dummy methods for typing reasons

func (p *PVT24) Start() error                    { panic("not implemented") }
func (p *PVT24) Stop() error                     { panic("not implemented") }
func (p *PVT24) WriteByte(b byte) error          { panic("not implemented") }
func (p *PVT24) ReadByte(ack bool) (byte, error) { panic("not implemented") }

func newPVT24(conf EEPROM24Config, t *testing.T) *PVT24 {
	var p PVT24
	p.mem = make([]byte, conf.Size)
	p.pagesize = conf.PageSize
	p.t = t

	for i := range p.mem {
		p.mem[i] = 0x24 ^ uint8(i)
	}

	return &p
}

func TestEEPROM24EOF(t *testing.T) {
	conf := Conf_24C02
	pvt := newPVT24(conf, t)
	ee, err := NewEEPROM24(pvt, Addr7(0xa0>>1), conf)
	if err != nil {
		t.Fatalf("NewEEPROM24 should not fail in this context. it did with % T: %#v\n", err, err)
	}

	_ee, ok := ee.(*ee24)
	if !ok {
		t.Fatalf("expected to test an ee24, got %T: %#v\n", ee, ee)
	}

	_ee.p = conf.Size - 3
	{
		rb := make([]byte, 16)
		n, err := ee.Read(rb)
		if n != 3 {
			t.Fatalf("expected to read 3 bytes, got %d\n", n)
		}

		if err != io.EOF {
			if err == nil {
				n, err := ee.Read(rb)

				if n != 0 {
					t.Fatalf("expected to read 0 bytes with the second shot, got %d\n", n)
				}

				if err != io.EOF {
					t.Fatalf("did not get back io.EOF even though EEPROM24 was asked twice, got: %T: %#v\n", err, err)
				}
			} else {
				t.Fatalf("expected EOF, got %T: %#v", err)
			}
		}
	}

	_ee.p = conf.Size - 5
	{
		wb := make([]byte, 16)
		n, err := ee.Write(wb)
		if n != 5 {
			t.Fatalf("expected to write 5 bytes, wrote %d\n", n)
		}

		if err != io.EOF {
			t.Fatalf("ee24.Write past EOF: expected EOF, got %T: %#v", err)
		}

		if _ee.p != conf.Size {
			t.Fatalf("ee24 file pointer at EOF should be at %#04x, however it is at %#04x", conf.Size, _ee.p)
		}

		n, err = ee.Write(wb)

		if n != 0 {
			t.Fatalf("expected to read 0 bytes with the second shot, got %d\n", n)
		}

		if err != io.EOF {
			t.Fatalf("ee24.Write at EOF: expected EOF, got: %T: %#v\n", err, err)
		}

		if _ee.p != conf.Size {
			t.Fatalf("ee24 file pointer at EOF should be at %#04x, however it is at %#04x", conf.Size, _ee.p)
		}
	}
}

func TestEEPROM24Conf(t *testing.T) {
	defconf := EEPROM24Config{1, 1, 0}
	devaddr := Addr7(0xA0 >> 1)
	tr := newPVT24(defconf, t)

	// one valid configuration as a counter check
	{
		if _, err := NewEEPROM24(tr, devaddr, defconf); err != nil {
			t.Errorf("NewEEPROM24 failed on valid configuration %#v\n", err)
		}
	}

	// pagesize not power of 2
	{
		conf := EEPROM24Config{2048, 13, 0}
		if _, err := NewEEPROM24(tr, devaddr, conf); err == nil {
			t.Errorf("NewEEPROM24 did not fail on invalid configuration %#v", conf)
		}
	}

	// size not power of 2
	{
		conf := EEPROM24Config{100, 16, 0}
		if _, err := NewEEPROM24(tr, devaddr, conf); err == nil {
			t.Errorf("NewEEPROM24 did not fail on invalid configuration %#v", conf)
		}
	}

	// size and page size not power of 2
	{
		conf := EEPROM24Config{100, 13, 0}
		if _, err := NewEEPROM24(tr, devaddr, conf); err == nil {
			t.Errorf("NewEEPROM24 did not fail on invalid configuration %#v", conf)
		}
	}

	// size too big
	{
		conf := EEPROM24Config{2 * MAX_EEPROM_SIZE, 16, 0}
		if _, err := NewEEPROM24(tr, devaddr, conf); err == nil {
			t.Errorf("NewEEPROM24 did not fail on invalid (size too big) configuration %#v\n", conf)
		}
	}
}

// input/output testing for Read and Write. as this test employs pvt24,
// the test will fail on transaction which ignore page size and the offset
// in the page.
func TestEEPROM24InOut(t *testing.T) {
	cases := []struct {
		conf   EEPROM24Config
		offs   uint // offset to seek to at the start
		read   bool
		buf    []byte // with read buf is just used to indicate the size, data is verified with the pvt24 mem pattern
		nexp   int
		errexp error
	}{ // small EEPROM configurations
		{EEPROM24Config{1024, 8, 0}, 6, true, []byte{0x22, 0x23, 0x2c, 0x2d, 0x2e, 0x2f}, 6, nil},
		{EEPROM24Config{128, 8, 0}, 123, true, []byte{0x5f, 0x58, 0x59, 0x5a, 0x5b, 0x00, 0x00, 0x00, 0x00}, 5, nil}, // double shot EOF returns err==nil on first call
		{EEPROM24Config{2048, 4, 0}, 9, false, []byte{0x0fe}, 1, nil},                                                // single byte write
		{EEPROM24Config{2048, 4, 0}, 2040, false, []byte{0xfc, 0xfd, 0xfe, 0xff}, 4, nil},                            // full page
		{EEPROM24Config{2048, 4, 0}, 513, false, []byte{0x01, 0x02, 0x03, 0x04}, 4, nil},                             // 1 byte in next page
		{EEPROM24Config{512, 4, 0}, 239, false, []byte{1, 2, 3, 4, 5, 6}, 6, nil},                                    // 1 byte partial, 4 bytes full, 1 byte partial
		{EEPROM24Config{512, 8, 0}, 254, false, []byte{1, 2, 3}, 3, nil},                                             // span i2c device boundary
		{EEPROM24Config{1024, 16, 0}, 1022, false, []byte{1, 2, 3, 4}, 2, io.EOF},                                    // test EOF. write employs a single shot EOF strategy
		// large EEPROM configurations
		{EEPROM24Config{1 << 16, 32, 0}, 6, true, []byte{0x22, 0x23, 0x2c, 0x2d, 0x2e, 0x2f}, 6, nil},
		{EEPROM24Config{1 << 16, 8, 0}, (1 << 16) - 5, true, []byte{0xdf, 0xd8, 0xd9, 0xda, 0xdb, 0x00, 0x00, 0x00, 0x00}, 5, nil}, // double shot EOF returns err==nil on first call
		{EEPROM24Config{1 << 16, 4, 0}, 9, false, []byte{0x0fe}, 1, nil},                                                           // single byte write
		{EEPROM24Config{1 << 16, 4, 0}, 2040, false, []byte{0xfc, 0xfd, 0xfe, 0xff}, 4, nil},                                       // full page
		{EEPROM24Config{1 << 16, 4, 0}, 513, false, []byte{0x01, 0x02, 0x03, 0x04}, 4, nil},                                        // 1 byte in next page
		{EEPROM24Config{1 << 16, 4, 0}, 239, false, []byte{1, 2, 3, 4, 5, 6}, 6, nil},                                              // 1 byte partial, 4 bytes full, 1 byte partial
		{EEPROM24Config{1 << 16, 8, 0}, 254, false, []byte{1, 2, 3}, 3, nil},                                                       // span i2c device boundary
		{EEPROM24Config{1 << 16, 16, 0}, (1 << 16) - 2, false, []byte{1, 2, 3, 4}, 2, io.EOF},                                      // test EOF. write employs a single shot EOF strategy
	}

	for i, c := range cases {
		devaddr := Addr7(0xA0 >> 1)
		tr := newPVT24(c.conf, t)

		ee, err := NewEEPROM24(tr, devaddr, c.conf)
		if err != nil {
			t.Errorf("NewEEPROM failed unexpectedly on configuration %T: %#v", c.conf, c.conf)
			continue
		}

		_ee := ee.(*ee24)
		_ee.p = c.offs

		var n int
		var rb []byte

		if c.read {
			rb = make([]byte, len(c.buf))
			n, err = ee.Read(rb)

			if n != c.nexp {
				t.Errorf("case %d: expected ee24 to read %d bytes, it read %d\n", i, c.nexp, n)
				continue
			}

			if err != c.errexp {
				t.Errorf("case %d: expected ee24.Read to return error %#v, it returned %T: %#v", i, c.errexp, err)
				continue
			}

			if n > len(c.buf) {
				t.Errorf("case %d: ee.Read read %d bytes, even though the slice only contains %d bytes", i, n, len(c.buf))
				continue
			}

			expp := c.offs + uint(c.nexp)
			if _ee.p != expp {
				t.Errorf("case %d: expected file pointer to be at %d at the end of ee.Read, it is at %d", i, expp, _ee.p)
				continue
			}

			if string(rb[0:n]) != string(c.buf[0:c.nexp]) {
				t.Errorf("case %d: ee24.Read is expected to read bytes % x, it read % x", i, c.buf[0:c.nexp], rb[0:n])
				continue
			}
		} else {
			n, err = ee.Write(c.buf)

			if n != c.nexp {
				t.Errorf("case %d: expected to write %d bytes, ee24 wrote %d", i, c.nexp, n)
				continue
			}

			if err != c.errexp {
				t.Error("case %d: expected error %#v, got %T: #%v", i, c.errexp, err, err)
				continue
			}

			expp := c.offs + uint(c.nexp)
			if _ee.p != expp {
				t.Errorf("case %d: expected file pointer to be at %d at the end of ee.Write, it its at %d", i, expp, _ee.p)
				continue
			}

			wmem := tr.mem[c.offs : c.offs+uint(c.nexp)]
			wbuf := c.buf[:c.nexp]
			if string(wmem) != string(wbuf) {
				t.Errorf("case %d: expected ee.Write to have written % x, it wrote % x", i, wbuf, wmem)
				continue
			}
		}
	}
}
