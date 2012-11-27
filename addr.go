package i2cm

type Addr interface {
	GetBaseAddr() uint16
	GetAddrLen() int
}

type Addr7 uint8

func (a Addr7) GetBaseAddr() uint16 {
	return uint16(a & 0x7f)
}

func (a Addr) GetAddrLen() int {
	return 7
}

type Addr10 uint16

func (a Addr10) GetBaseAddr() uint16 {
	return uint16(a & 0x03ff)
}

func (a Addr10) GetAddrLen() {
	return 10
}
