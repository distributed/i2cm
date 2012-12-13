// Copyright 2012 Michael Meier. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package i2cm

import "errors"

// NACKReceived signals that devices did not ACK.
var NACKReceived = errors.New("NACK received")

// NoSuchDevice signals that no device responded
// with an ACK at the desired address.
var NoSuchDevice = errors.New("no such device")
