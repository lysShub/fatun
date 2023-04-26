package raw

import (
	"errors"
	"itun/pack"
	"net"
	"net/netip"
)

type RawConn interface {
	WriteTo(prot uint8, b []byte, dst netip.Addr) (int, error)
}

func NewRawConn(locAddr netip.Addr) (RawConn, error) {

	if !locAddr.Is6() {
		return newIP4Conn(locAddr)
	} else {
		return nil, errors.New("only support IPv4")
	}
}

type ip4Raw struct {
	tcpConn *net.IPConn
	udpConn *net.IPConn
}

func newIP4Conn(laddr netip.Addr) (*ip4Raw, error) {
	var r = &ip4Raw{}
	var err error
	r.udpConn, err = net.ListenIP("ip4:udp", &net.IPAddr{IP: laddr.AsSlice()})
	if err != nil {
		return nil, err
	}
	r.tcpConn, err = net.ListenIP("ip4:tcp", &net.IPAddr{IP: laddr.AsSlice()})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (c *ip4Raw) WriteTo(prot uint8, b []byte, dst netip.Addr) (int, error) {
	switch prot {
	case pack.TCP:
		return c.tcpConn.WriteToIP(b, &net.IPAddr{IP: dst.AsSlice(), Zone: dst.Zone()})
	case pack.UDP:
		return c.udpConn.WriteToIP(b, &net.IPAddr{IP: dst.AsSlice(), Zone: dst.Zone()})
	default:
		return 0, errors.New("not support protocol, only support TCP and UDP")
	}
}
