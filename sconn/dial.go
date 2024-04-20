package sconn

import (
	"context"

	"github.com/lysShub/fatun/session"
	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/sockit/conn"
)

// todo: 应该在newConn就应该确定crypto等只资源类型，起码确定其Overhead()的值，这样
// Conn的Overhead大小是确定的，在link.NewXxx使用cfg.HandshakeMTU-c.Overhead()。
// 现在使用的是maxOverhead是理论最大开销。
const maxOverhead = 20 + 16 + session.Size

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
		link.NewList(8, cfg.HandshakeMTU-maxOverhead),
		raw.LocalAddr().Addr(),
	)
	if err != nil {
		return nil, err
	}
	// stack = test.MustWrapPcap("client.pcap", stack)

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
