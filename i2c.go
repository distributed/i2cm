package i2cm

type I2CMaster interface {
	Start() error
	Stop() error
	ReadByte(ack bool) (recvb byte, err error)
	WriteByte(b byte) error
}
