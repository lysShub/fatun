package pack

import "unsafe"

const W = 4

func Packe(b []byte, dstIP [4]byte) []byte {
	n := len(b)
	if n+W < cap(b) {
		tb := make([]byte, n+W)
		copy(tb[0:], b[0:])
		b = tb
	} else {
		b = b[:n+W]
	}

	*(*[4]byte)(unsafe.Pointer(&b[n])) = dstIP
	return b
}

func Parse(b []byte) (ip [4]byte) {
	var _ = b[5]

	n := len(b) - W
	return *(*[4]byte)(unsafe.Pointer(&b[n]))
}

func Checksum(d [20]byte) uint16 {
	var S uint32
	for i := 0; i < 20; i = i + 2 {
		S = S + uint32(d[i])<<8 + uint32(d[i+1])
		if S>>16 > 0 {
			S = S&0xffff + 1
		}
	}

	return uint16(65535) - uint16(S)
}
