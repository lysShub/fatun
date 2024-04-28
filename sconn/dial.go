package sconn

import (
	"context"

	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/sockit/conn"
)

func Dial(raw conn.RawConn, cfg *Config) (*Conn, error) {
	return DialCtx(context.Background(), raw, cfg)
}

func DialCtx(ctx context.Context, raw conn.RawConn, cfg *Config) (*Conn, error) {
	return dial(ctx, raw, cfg)
}

func dial(ctx context.Context, raw conn.RawConn, cfg *Config) (*Conn, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}

	stack, err := ustack.NewUstack(
		link.NewList(8, cfg.MTU-Overhead),
		raw.LocalAddr().Addr(),
	)
	if err != nil {
		return nil, err
	}
	// stack = test.MustWrapPcap("client-ctr.pcap", stack)

	ep, err := stack.LinkEndpoint(raw.LocalAddr().Port(), raw.RemoteAddr())
	if err != nil {
		return nil, err
	}

	conn, err := newConn(raw, ep, client, cfg)
	if err != nil {
		return nil, conn.close(err)
	}
	conn.factory = &clientFactory{
		local: conn.LocalAddr(), remote: conn.RemoteAddr(),
		stack: stack,
	}

	if err = conn.handshake(ctx); err != nil {
		return nil, conn.close(err)
	}
	return conn, nil
}
