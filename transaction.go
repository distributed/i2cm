package i2cm

import (
	"errors"
)

type Transactor8x8 interface {
	Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) error // TODO: nwrriten, nread?
}

type transactor8x8 struct {
	m I2CMaster
}

func (t transactor8x8) Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) error {
	m := t.m

	if addr.GetAddrLen() != 7 {
		return errors.New("only 7 bit addresses are supported")
	}

	if err := m.Start(); err != nil {
		return err
	}

	addrb := uint8(addr.GetBaseAddr() << 1)
	if err := m.WriteByte(addrb); err != nil {
		if err == NACKReceived {
			return NoSuchDevice
		}
		return err
	}

	// write regaddr
	// write w
	// start again
	// write device addr
	// read into r
}
