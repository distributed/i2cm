// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package i2cm

import (
	"errors"
)

// Transactor encompasses all implemented I2C bus transaction
// types.
type Transactor interface {
	Transactor8x8
	Transactor16x8
}

type transactor struct {
	Transactor8x8
	Transactor16x8
}

// NewTransactor returns all implemented I2C transactors
// based on the argument I2CMaster. This is a convenience
// function which consolidates the results of the 
// NewTransact*x* family of functions.
func NewTransactor(m I2CMaster) Transactor {
	var t transactor
	t.Transactor8x8 = NewTransact8x8(m)
	t.Transactor16x8 = NewTransact16x8(m)

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

// Implements a write-then-read transaction with 16 bit register
// addresses and 8 bit data. The transaction always writes data
// to the device, as the register address is always written.
// The read part of the transaction is not executed if len(r) == 0.
// nw and nr specify the number of bytes written or read,
// respectively, before an error occured or the transaction finished.
// If err == nil, then nw == len(w) and nr == len(r).
//
// A transaction with len(r) == 0 is carried out as follows:
// 		[S] [(devaddr<<1)] A [hi8(regaddr)] A [lo8(regaddr)] A [w[0]] A ... [P]
// 
// A transaction with len(r) is carried out as follows:
// 		[S] [(devaddr<<1)] A [hi8(regaddr)] A [lo8(regaddr)] A [w[0]] A ... [S] [(devaddr<<1)|1] r[0] [A] ... r[len(r)-1] [N] [P]
type Transactor16x8 interface {
	Transact16x8(addr Addr, regaddr uint16, w []byte, r []byte) (nw, nr int, err error)
}

type transactor16x8 struct {
	tr8x8 Transactor8x8
}

// NewTransact16x8 returns a Transactor16x8 which is based on m.
// If the argument m is already a Transactor16x8, it returns
// the underlying Transactor16x8. If not, NewTransact16x8 creates a
// a Transactor8x8 from the supplied I2C Master and uses it to emulate
// 16x8 accesses.
//
// In contrast to NewTransact8x8, there is no low-level implementation
// of a 16x8 transaction in this package.
func NewTransact16x8(m I2CMaster) Transactor16x8 {
	if t, ok := m.(Transactor16x8); ok {
		return t
	}
	return transactor16x8{NewTransact8x8(m)}
}

func (t transactor16x8) Transact16x8(addr Addr, regaddr uint16, w []byte, r []byte) (int, int, error) {
	// we emulate a 16x8 transaction by doing an 8x8 transaction with hi8(regaddr)
	// as the "register address" and lo8(regaddr) as the first byte to write
	addrhi := uint8(regaddr >> 8)
	wbuf := make([]byte, 0, 1+len(w))
	wbuf = append(wbuf, uint8(regaddr))
	wbuf = append(wbuf, w...)
	rbuf := r

	nw, nr, err := t.tr8x8.Transact8x8(addr, addrhi, wbuf, rbuf)
	// adjust for the one byte of "data" which, in fact, was the lower byte
	// of the register address
	if nw > 0 {
		nw--
	}
	return nw, nr, err
}
