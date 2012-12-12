package i2cm

import (
	"errors"
)

// Transactor encompasses all implemented I2C bus transaction
// types.
type Transactor interface {
	Transactor8x8
}

type transactor struct {
	Transactor8x8
}

// NewTransactor returns all implemented I2C transactors
// based on the argument I2CMaster. This is a convenience
// function which consolidates the results of the 
// NewTransact*x* family of functions.
func NewTransactor(m I2CMaster) Transactor {
	var t transactor
	t.Transactor8x8 = NewTransact8x8(m)

	return &t
}

type Transactor8x8 interface {
	Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) (nw, nr int, err error)
}

type transactor8x8 struct {
	m I2CMaster
}

func NewTransact8x8(m I2CMaster) Transactor8x8 {
	if t, ok := m.(Transactor8x8); ok {
		return t
	}
	return transactor8x8{m}
}

func (t transactor8x8) Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) (int, int, error) {
	return I2CMasterTransact8x8(t.m, addr, regaddr, w, r)
}

func I2CMasterTransact8x8(m I2CMaster, addr Addr, regaddr uint8, w []byte, r []byte) (int, int, error) {
	nr := 0
	nw := 0

	if addr.GetAddrLen() != 7 {
		return nw, nr, errors.New("I2CMasterTransact8x8: only 7 bit addresses are supported")
	}

	if err := m.Start(); err != nil {
		return nw, nr, err
	}

	// inner function handles the whole transaction between
	// but not including the start and the stop bit
	err := func() error {
		// address device
		addrb := uint8(addr.GetBaseAddr() << 1)
		if err := m.WriteByte(addrb); err != nil {
			if err == NACKReceived {
				return NoSuchDevice
			}
			return err
		}

		// write regaddr
		if err := m.WriteByte(regaddr); err != nil {
			return err
		}

		// write w
		for _, b := range w {
			if err := m.WriteByte(b); err != nil {
				return err
			}

			nw++
		}

		// read part of transaction is only performed if desired
		if len(r) > 0 {
			// start again
			if err := m.Start(); err != nil {
				return err
			}

			// write device's read address
			if err := m.WriteByte(addrb | 0x01); err != nil {
				if err == NACKReceived {
					return NoSuchDevice
				}
				return err
			}

			for i := 0; i < len(r); i++ {
				ack := true
				if i == len(r)-1 {
					ack = false
				}
				rb, err := m.ReadByte(ack)
				if err != nil {
					return err
				}

				r[i] = rb

				nr++
			}

		}

		return nil
	}()

	if err != nil {
		// if there already was an error, the error from stop is ignored
		// and the first error is reported
		m.Stop()
	} else {
		err = m.Stop()
	}

	return nw, nr, err
}
