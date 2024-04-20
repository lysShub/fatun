package control

import (
	"context"
	"net"

	"github.com/lysShub/fatun/session"
)

type Handler interface {
	InitConfig(cfg *Config) error
	AddSession(s session.Session) (session.ID, error)
	DelSession(id session.ID) error
	PackLoss() float32
	Ping()
}

type Config struct {
	MSS int
}

type Client interface {
	Close() error

	InitConfig(ctx context.Context, cfg *Config) error
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

func Serve(ctx context.Context, conn net.Conn, hdr Handler) error {
	return NewServer(conn, hdr).Serve(ctx)
}
