package sconn

import (
	"context"

	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/rsocket"
)

func Dial(raw rsocket.RawConn, cfg *Config) (*Conn, error) {
	return DialCtx(context.Background(), raw, cfg)
}

func DialCtx(ctx context.Context, raw rsocket.RawConn, cfg *Config) (*Conn, error) {
	link := link.WrapNofin(link.NewList(8, cfg.MTU))
	stack, err := ustack.NewUstack(link, raw.LocalAddr().Addr())
	if err != nil {
		return nil, err
	}

	conn, err := newConn(raw, client, cfg)
	if err != nil {
		return nil, err
	}

	err = conn.handshakeConnect(ctx, stack)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
