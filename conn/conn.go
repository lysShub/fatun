package conn

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/netkit/packet"
)

type Listener interface {
	Accept() (Conn, error)
	Addr() netip.AddrPort
	Close() error
}

// datagram conn
type Conn interface {

	// BuiltinConn get builtin stream connect, require Recv be called async.
	BuiltinConn(ctx context.Context) (conn net.Conn, err error)

	Recv(peer Peer, payload *packet.Packet) (err error)
	Send(peer Peer, payload *packet.Packet) (err error)

	LocalAddr() netip.AddrPort
	RemoteAddr() netip.AddrPort
	Close() error
}
