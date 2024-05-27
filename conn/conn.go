package conn

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/netkit/packet"
)

type Listener interface {
	Accept() (Conn, error)
	AcceptCtx(ctx context.Context) (Conn, error)
	MTU() int
	Addr() netip.AddrPort
	Close() error
}

// datagram conn
type Conn interface {
	BuiltinTCP(ctx context.Context) (tcp net.Conn, err error)
	Recv(atter Peer, payload *packet.Packet) (err error)
	Send(atter Peer, payload *packet.Packet) (err error)

	LocalAddr() netip.AddrPort
	RemoteAddr() netip.AddrPort
	Close() error
}
