package i2cm

import "errors"

var NACKReceived = errors.New("NACK received")

var NoSuchDevice = errors.New("no such device")
