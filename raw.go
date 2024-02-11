package itun

import (
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type RawConn struct {
	relraw.RawConn
}

func WrapRawConn(conn relraw.RawConn) *RawConn {
	return &RawConn{RawConn: conn}
}

func (r *RawConn) LocalAddr() tcpip.FullAddress {
	return tcpip.FullAddress{Addr: tcpip.AddrFromSlice(r.LocalAddrAddrPort().Addr().AsSlice()), Port: r.LocalAddrAddrPort().Port()}
}

func (r *RawConn) RemoteAddr() tcpip.FullAddress {
	return tcpip.FullAddress{Addr: tcpip.AddrFromSlice(r.LocalAddrAddrPort().Addr().AsSlice()), Port: r.LocalAddrAddrPort().Port()}
}

func (r *RawConn) NetworkProtocolNumber() tcpip.NetworkProtocolNumber {
	addr := r.LocalAddrAddrPort().Addr()
	if addr.Is4() {
		return header.IPv4ProtocolNumber
	} else if addr.Is6() {
		return header.IPv6ProtocolNumber
	} else {
		return 0
	}
}

func (r *RawConn) MTU() int {
	return 0
}
