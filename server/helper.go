package server

import (
	"fmt"
	"net"
	"net/netip"
)

var _nullAddrPort = netip.AddrPort{}

func toNetIP(addr net.Addr) (netip.AddrPort, error) {
	switch addr.(type) {
	case *net.IPAddr:
		addr, err := netip.ParseAddr(addr.String())
		if err != nil {
			return _nullAddrPort, err
		}
		return netip.AddrPortFrom(addr, 0), nil
	case *net.TCPAddr, *net.UDPAddr:
		return netip.ParseAddrPort(addr.String())
	default:
		return _nullAddrPort, fmt.Errorf("unsupported address type %T", addr)
	}
}
