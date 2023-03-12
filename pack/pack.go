package pack

/*
	代理的是应用层数据包, 是无连接代理, 所以在
	每个数据包后附加上dstIP.
*/

import "unsafe"

const W = 6

func Packe(b []byte, dstIP [4]byte, localPort uint16) []byte {
	n := len(b)
	if n+W < cap(b) {
		tb := make([]byte, n+W)
		copy(tb[0:], b[0:])
		b = tb
	} else {
		b = b[:n+W]
	}

	*(*[4]byte)(unsafe.Pointer(&b[n])) = dstIP
	*(*uint16)(unsafe.Pointer(&b[n+4])) = localPort
	return b
}

func Parse(b []byte) (ip [4]byte, localPort uint16) {
	var _ = b[5]

	n := len(b) - W
	ip = *(*[4]byte)(unsafe.Pointer(&b[n]))
	localPort = *(*uint16)(unsafe.Pointer(&b[n+4]))
	return
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
