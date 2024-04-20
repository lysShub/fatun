package app_test

import (
	"net/netip"

	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

var (
	caddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{
		// 172, 25, 32, 1,
		172, 24, 128, 1,
	}), 19986)

	saddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{
		// 172, 25, 38, 4,
		172, 24, 131, 26,
	}), 8080)
)

var sign = &sconn.Sign{
	Sign: []byte("0123456789abcdef"),
	Parser: func(sign []byte) (crypto.Key, error) {
		return crypto.Key{9: 1}, nil
	},
}

var pps = sconn.PrevPackets{
	header.TCP("hello"),
	header.TCP("world"),
}
