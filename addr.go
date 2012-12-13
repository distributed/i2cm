// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package i2cm

// Addr represents an I2C device address. It supports both 7 bit and
// 10 bit addressing. Other address width are possible but will not
// be supported by transaction drivers.
type Addr interface {
	// Get the device address. This does not encompass the R/W bit.
	GetBaseAddr() uint16

	// Get the device address' width in bits. Reasonable values are
	// 7 and 10 bits.
	GetAddrLen() int
}

// Addr7 represents a 7 bit I2C address. The device address must be
// right aligned.
type Addr7 uint8

func (a Addr7) GetBaseAddr() uint16 {
	return uint16(a & 0x7f)
}

func (a Addr7) GetAddrLen() int {
	return 7
}

// Addr7 represents a 10 bit I2C address. The device address must be
// right aligned.
type Addr10 uint16

func (a Addr10) GetBaseAddr() uint16 {
	return uint16(a & 0x03ff)
}

func (a Addr10) GetAddrLen() int {
	return 10
}
