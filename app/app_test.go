package app_test

import (
	"net/netip"
	"time"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
	"github.com/pkg/errors"
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

	ht = time.Hour
)

type tkClient struct{}

func (c *tkClient) Token() (tk []byte, key crypto.Key, err error) {
	return []byte("hello"), crypto.Key{1: 1}, nil
}

type tkServer struct{}

func (c *tkServer) Valid(tk []byte) (key crypto.Key, err error) {
	if string(tk) == "hello" {
		return crypto.Key{1: 1}, nil
	}
	return crypto.Key{}, errors.Errorf("invalid token")
}

var pps = sconn.PrevPackets{
	header.TCP("hello"),
	header.TCP("world"),
}
