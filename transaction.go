package i2cm

import (
	"errors"
)

type Transactor8x8 interface {
	Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) (nw, nr int, err error)
}

type transactor8x8 struct {
	m I2CMaster
}

func NewTransact8x8(m I2CMaster) Transactor8x8 {
	return transactor8x8{m}
}

func (t transactor8x8) Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) (int, int, error) {
	m := t.m

	nr := 0
	nw := 0

	if addr.GetAddrLen() != 7 {
		return nw, nr, errors.New("Transact8x8: only 7 bit addresses are supported")
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
