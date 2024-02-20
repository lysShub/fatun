package itun

import (
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type RawConn struct {
	relraw.RawConn
	mtu uint16
}

func WrapRawConn(conn relraw.RawConn, mtu uint16) *RawConn {
	return &RawConn{RawConn: conn, mtu: mtu}
}

func (r *RawConn) LocalAddr() tcpip.FullAddress {
	return tcpip.FullAddress{
		Addr: tcpip.AddrFromSlice(r.LocalAddrPort().Addr().AsSlice()),
		Port: r.LocalAddrPort().Port(),
	}
}

func (r *RawConn) RemoteAddr() tcpip.FullAddress {
	return tcpip.FullAddress{
		Addr: tcpip.AddrFromSlice(r.RemoteAddrPort().Addr().AsSlice()),
		Port: r.RemoteAddrPort().Port(),
	}
}

func (r *RawConn) NetworkProtocolNumber() tcpip.NetworkProtocolNumber {
	addr := r.LocalAddrPort().Addr()
	if addr.Is4() {
		return header.IPv4ProtocolNumber
	} else if addr.Is6() {
		return header.IPv6ProtocolNumber
	} else {
		return 0
	}
}

func (r *RawConn) MTU() int {
	return int(r.mtu)
}
