package pack

import (
	"net/netip"
	"unsafe"
)

type Proto = uint8

const (
	ICMP Proto = 1
	TCP        = 6
	UDP        = 17
)

type Pack interface {
	Encode(b []byte, proto Proto, remoteAddr netip.Addr) []byte
	Decode(b []byte) (n int, proto Proto, remoteAddr netip.Addr)
}

const W = 4 + 1 // only ipv4

type pack struct{}

func (p *pack) Encode(b []byte, proto Proto, remoteAddr netip.Addr) []byte {
	n := len(b)
	if n+W < cap(b) {
		tb := make([]byte, n+W)
		copy(tb[0:], b[0:])
		b = tb
	} else {
		b = b[:n+W]
	}

	*(*[4]byte)(unsafe.Pointer(&b[n])) = netip.MustParseAddr(remoteAddr.String()).As4()
	b[n+4] = byte(proto)
	return b
}

func (p *pack) Decode(b []byte) (n int, proto Proto, remoteAddr netip.Addr) {
	n = len(b)
	if n < W {
		return
	}

	proto = Proto(b[n-1])
	remoteAddr = netip.AddrFrom4([4]byte(b[n-W : n-1]))
	n = n - W

	return n, proto, remoteAddr
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
