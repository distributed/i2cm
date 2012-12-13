// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package i2cm

import (
	"errors"
	"io"
	"time"
)

// EEPROM24Config is used to configure the EEPROM driver to use a
// specific device.
type EEPROM24Config struct {
	Size       uint
	PageSize   uint
	WriteDelay time.Duration // time to wait after a page write. Address polling is not implemented
}

var Conf_24C02 = EEPROM24Config{256, 8, 5 * time.Millisecond}

// ee24 supports EEPROMs which require an 8 bit word
// address. IIRC, this includes types up to 24c16.
// 24c32 devices and up require a 16 word address.
type ee24 struct {
	EEPROM24Config
	m       I2CMaster
	p       uint // file pointer
	devaddr Addr
}

// EEPROM24 represents an I2C EEPROM device. The memory array is made
// available via a file-like interface. The file's size is fixed to
// the memory array size and writes past the end of the array result
// in an error.
type EEPROM24 interface {
	io.Reader
	io.Seeker
	io.Writer
}

func ispow2(i uint64) bool {
	for (i&0x01) == 0 && i > 0 {
		i >>= 1
	}
	return i == 1
}

// NewEEPROM24 constructs an I2C EEPROM driver for a device with base
// address devaddr residing on m's bus. The EEPROM driver parameters
// are passed in conf. Invalid configurations are rejected.
//
// The EEPROM driver currently uses Transactor8x8 and thus only works
// correctly for devices with 8 bit register addresses.
func NewEEPROM24(m I2CMaster, devaddr Addr, conf EEPROM24Config) (EEPROM24, error) {
	if conf.PageSize > conf.Size {
		return nil, errors.New("EEPROM24: page size needs to be smaller than array size")
	}

	if !ispow2(uint64(conf.Size)) {
		return nil, errors.New("EEPROM24: array size needs to be a power of 2")
	}

	if !ispow2(uint64(conf.PageSize)) {
		return nil, errors.New("EEPROM24: page size needs to be a power of 2")
	}

	if devaddr.GetAddrLen() != 7 {
		return nil, errors.New("only EEPROMs with 7 bit device addresses are supported")
	}

	var e ee24

	e.m = m
	e.EEPROM24Config = conf
	e.p = 0
	e.devaddr = devaddr

	return &e, nil
}

func (e *ee24) Read(b []byte) (int, error) {
	// TODO: does read address roll over at the end of the
	// memory array or every 256 bytes?

	startpos := e.p
	endpos := startpos + uint(len(b))
	if endpos > e.Size {
		endpos = e.Size
	}

	if endpos-startpos == 0 {
		return 0, io.EOF
	}

	devaddrinc := startpos >> 8 // 256 byte every 1 7-bit slave addr
	devaddr := Addr7(uint8(e.devaddr.GetBaseAddr() + uint16(devaddrinc)))

	rb := b[0:(endpos - startpos)]

	regaddr := uint8(startpos & 0xff)

	t := NewTransact8x8(e.m)
	_, nr, err := t.Transact8x8(devaddr, regaddr, nil, rb)

	e.p += uint(nr)

	return nr, err
}

func (e *ee24) Seek(offset int64, whence int) (int64, error) {
	P := int64(e.p)

	// this may fail in funny ways for big absolute values of offset.
	var nP int64
	switch whence {
	case 0:
		nP = offset
	case 1:
		nP = P + offset
	case 2:
		nP = int64(e.Size) + offset
	default:
		return P, errors.New("EEPROM24.Seek: invalid whence")
	}

	if nP < 0 {
		return P, errors.New("EEPROM24.Seek: negative position")
	}

	if nP > int64(e.Size) {
		return P, errors.New("EEPROM24.Seek: desired position beyond end of EEPROM array")
	}

	e.p = uint(nP)

	return P, nil
}

func (e *ee24) Write(b []byte) (int, error) {
	origsize := len(b)

	for len(b) > 0 && e.p < e.Size {
		regaddr := uint8(e.p & 0xff)
		devaddrinc := e.p >> 8 // 256 byte every 1 7-bit slave addr
		devaddr := Addr7(uint8(e.devaddr.GetBaseAddr() + uint16(devaddrinc)))

		// address in page
		aip := e.p & (e.PageSize - 1)
		//log.Printf("e.p %#04x  aip %#02x\n", e.p, aip)
		// get number of bytes to write in this page
		nip := uint(len(b))
		if nip > e.PageSize-aip {
			nip = e.PageSize - aip
		}

		// do transaction
		//log.Printf("at p %#04x, pagesize %#02x read nip %#02x\n", e.p, e.PageSize, nip)
		nw, _, err := NewTransact8x8(e.m).Transact8x8(devaddr, regaddr, b[0:nip], nil)
		if err != nil {
			return origsize - len(b) + nw, err
		}

		// TODO: either wait or poll for device

		e.p += uint(nip)
		b = b[nip:]
	}

	//log.Printf("at end of write, p %d  len(b) %d\n", e.p, len(b))

	if e.p == e.Size {
		// reached the end of the array
		if len(b) > 0 {
			return origsize - len(b), io.EOF
		}
	}
	if e.p > e.Size {
		panic("wrote beyond end of EEPROM. is the configuration correct?")
	}

	return origsize, nil
}
