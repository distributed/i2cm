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

// Implements a write-then-read transaction with 8 bit register
// addresses and 8 bit data. The transaction always writes data
// to the device, as the register address is always written.
// The read part of the transaction is not executed if len(r) == 0.
// nw and nr specify the number of bytes written or read,
// respectively, before an error occured or the transaction finished.
// If err == nil, then nw == len(w) and nr == len(r).
//
// A transaction with len(r) == 0 is carried out as follows:
// 		[S] [(devaddr<<1)] A [regaddr] A [w[0]] A ... [P]
// 
// A transaction with len(r) is carried out as follows:
// 		[S] [(devaddr<<1)] A [regaddr] A [w[0]] A ... [S] [(devaddr<<1)|1] r[0] [A] ... r[len(r)-1] [N] [P]
type Transactor8x8 interface {
	Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) (nw, nr int, err error)
}

type transactor8x8 struct {
	m I2CMaster
}

// NewTransact8x8 returns a Transactor8x8 which is based on m.
// If the argument m is already a Transactor8x8, it returns
// the underlying Transactor8x8. If you want to make sure that
// transactions are carried out using the low level I2CMaster
// interface, see I2CMasterTransact8x8.
func NewTransact8x8(m I2CMaster) Transactor8x8 {
	if t, ok := m.(Transactor8x8); ok {
		return t
	}
	return transactor8x8{m}
}

func (t transactor8x8) Transact8x8(addr Addr, regaddr uint8, w []byte, r []byte) (int, int, error) {
	return I2CMasterTransact8x8(t.m, addr, regaddr, w, r)
}

// I2CMasterTransact8x8 carries out a transaction as specified by
// Transactor8x8 by using the low level I2CMaster interface. This
// function can be used as a fallback for implementors of Transactor8x8
// in case their I2C bus master only supports a limited set of 8x8
// transactions.
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
