package conn

import (
	"context"
	"fmt"
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
	Recv(atter Session, payload *packet.Packet) (err error)
	Send(atter Session, payload *packet.Packet) (err error)

	MTU() int
	Role() Role
	Overhead() int
	LocalAddr() netip.AddrPort
	RemoteAddr() netip.AddrPort
	Close() error
}

type Role uint8

const (
	Client Role = 1
	Server Role = 2
)

func (r Role) Client() bool { return r == Client }
func (r Role) Server() bool { return r == Server }
func (r Role) String() string {
	switch r {
	case Client:
		return "client"
	case Server:
		return "server"
	default:
		return fmt.Sprintf("invalid fatcp role %d", r)
	}
}
