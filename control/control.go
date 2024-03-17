package control

import (
	"context"
	"net"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
)

type Handler interface {
	IPv6() bool
	EndConfig()
	AddSession(s session.Session) (session.ID, error)
	DelSession(id session.ID) error
	PackLoss() float32
	Ping()
}

type Client interface {
	Close() error

	IPv6(ctx context.Context) (bool, error)
	EndConfig(ctx context.Context) error

	AddSession(ctx context.Context, s session.Session) (*AddSession, error)
	DelSession(ctx context.Context, id session.ID) error

	PackLoss(ctx context.Context) (float32, error)
	Ping(ctx context.Context) error
}

func NewClient(conn net.Conn) Client {
	return newGobClient(conn)
}

type Server interface {
	Serve(context.Context) error
	Close() error
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
