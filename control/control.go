package control

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
)

type Client interface {
	Close() error

	IPv6(ctx context.Context) (bool, error)
	EndConfig(ctx context.Context) error
	AddTCP(ctx context.Context, addr netip.AddrPort) (*AddTCP, error)
	DelTCP(ctx context.Context, id session.ID) error
	AddUDP(ctx context.Context, addr netip.AddrPort) (*AddUDP, error)
	DelUDP(ctx context.Context, id session.ID) error
	PackLoss(ctx context.Context) (float32, error)
	Ping(ctx context.Context) error
}

func NewClient(conn net.Conn) Client {
	return newGobClient(conn)
}

type Server interface {
	Serve(context.Context) error
}

func NewServer(conn net.Conn, hdr Handler) Server {
	return newGobServer(conn, hdr)
}

func Serve(ctx cctx.CancelCtx, conn net.Conn, hdr Handler) {
	err := NewServer(conn, hdr).Serve(ctx)
	if err != nil {
		ctx.Cancel(err)
	}
}
