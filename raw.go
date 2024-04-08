package itun

import (
	"github.com/lysShub/sockit/conn"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type RawConn struct {
	conn.RawConn
	mtu uint16
}

func WrapRawConn(conn conn.RawConn, mtu uint16) *RawConn {
	return &RawConn{RawConn: conn, mtu: mtu}
}

func (r *RawConn) LocalAddr() tcpip.FullAddress {
	return tcpip.FullAddress{
		Addr: tcpip.AddrFromSlice(r.RawConn.LocalAddr().Addr().AsSlice()),
		Port: r.RawConn.LocalAddr().Port(),
	}
}

func (r *RawConn) RemoteAddr() tcpip.FullAddress {
	return tcpip.FullAddress{
		Addr: tcpip.AddrFromSlice(r.RawConn.RemoteAddr().Addr().AsSlice()),
		Port: r.RawConn.RemoteAddr().Port(),
	}
}

func (r *RawConn) NetworkProtocolNumber() tcpip.NetworkProtocolNumber {
	addr := r.RawConn.LocalAddr().Addr()
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
