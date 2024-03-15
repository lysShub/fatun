package sconn

import (
	"context"

	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/relraw"
)

func Dial(raw relraw.RawConn, cfg *Config) (*Sconn, error) {
	return DialCtx(context.Background(), raw, cfg)
}

func DialCtx(ctx context.Context, raw relraw.RawConn, cfg *Config) (*Sconn, error) {
	link := link.WrapNofin(link.NewList(8, int(cfg.MTU)))
	stack, err := ustack.NewUstack(link, raw.LocalAddrPort().Addr())
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
