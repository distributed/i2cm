// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package i2cm defines low level I2C master access and implements I2C
// transactions and an 24Cxx EEPROM driver.
package i2cm

// I2CMaster offers low-level access to an I2C bus.
// The driver implementing this interface must be
// the only master on the bus.
type I2CMaster interface {
	// Start sends a start or repeated start condition,
	// depending on bus state.
	Start() error

	// Sends a stop condition.
	Stop() error

	// ReadByte reads one byte and sends an ACK
	// if ack is true. Application code is responsible
	// to ensure that ack is false for the last read
	// before a stop bit is sent.
	ReadByte(ack bool) (recvb byte, err error)

	// WriteByte writes one byte to the device. If
	// device does not ACK, it returns NACKReceived.
	WriteByte(b byte) error
}
