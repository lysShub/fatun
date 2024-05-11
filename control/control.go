package control

import (
	"context"
	"net"

	sconn "github.com/lysShub/fatcp"
	"github.com/lysShub/fatun"
)

type Handler interface {
	InitConfig(cfg *Config) error
	AddSession(s fatun.Session) (sconn.Peer, error)
	DelSession(id sconn.Peer) error
	PackLoss() float32
	Ping()
}

type Config struct {
	MSS int
}

type Client interface {
	Close() error

	InitConfig(ctx context.Context, cfg *Config) error
	AddSession(ctx context.Context, s fatun.Session) (*AddSession, error)
	DelSession(ctx context.Context, id sconn.Peer) error
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
