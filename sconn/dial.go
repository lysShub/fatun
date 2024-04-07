package sconn

import (
	"context"

	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/sockit/conn"
)

const overhead = 20 + 16 + session.Size

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
		link.NewList(8, cfg.HandshakeMTU-overhead),
		raw.LocalAddr().Addr(),
	)
	if err != nil {
		return nil, err
	}
	// stack = utest.MustWrapPcap("ustack.pcap", stack)

	ep, err := ustack.ToEndpoint(stack, raw.LocalAddr().Port(), raw.RemoteAddr())
	if err != nil {
		return nil, err
	}

	conn, err := newConn(raw, ep, client, cfg)
	if err != nil {
		return nil, conn.close(err)
	}

	if err = conn.handshakeClient(ctx, stack); err != nil {
		return nil, conn.close(err)
	}
	return conn, nil
}
