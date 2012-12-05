package i2cm

import (
	"io"
	"testing"
)

type t8x8item struct {
	addr    Addr
	regaddr uint8
	wb, rb  []byte
	nw, nr  int
	err     error
}

// page verifying transactor for 24Cxx style EEPROMs
// it's always based at address (0xA0 >> 1).
// also logs.
type PVT24 struct {
	t        *testing.T
	mem      []byte
	pagesize uint
	log      []t8x8item
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

	p.log = append(p.log, t8x8item{addr, regaddr, wb, rb, len(wb), len(rb), nil})

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
		p.mem[i] = 0x24
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
			if err == nil {
				n, err := ee.Write(wb)

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
}
