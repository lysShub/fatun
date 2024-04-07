package sconn

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	utest "github.com/lysShub/itun/ustack/test"
	"github.com/lysShub/sockit/conn"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Listener struct {
	cfg *Config
	raw conn.Listener

	stack ustack.Ustack
	l     *gonet.TCPListener
}

func NewListener(l conn.Listener, cfg *Config) (*Listener, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}

	stack, err := ustack.NewUstack(link.NewList(64, cfg.HandshakeMTU-overhead), l.Addr().Addr())
	if err != nil {
		return nil, err
	}
	stack = utest.MustWrapPcap("ustack.pcap", stack)

	listener, err := gonet.ListenTCP(stack, l.Addr(), header.IPv4ProtocolNumber)
	if err != nil {
		return nil, err
	}

	return &Listener{
		cfg:   cfg,
		raw:   l,
		stack: stack,
		l:     listener,
	}, nil
}

func (l *Listener) Accept() (*Conn, error) {
	return l.AcceptCtx(context.Background())
}

func (l *Listener) AcceptCtx(ctx context.Context) (*Conn, error) {
	raw, err := l.raw.Accept() // todo: raw support context
	if err != nil {
		return nil, err
	}

	ep, err := ustack.NewEndpoint(l.stack, l.Addr().Port(), raw.RemoteAddr())
	if err != nil {
		return nil, err
	}
	conn, err := newConn(raw, ep, server, l.cfg)
	if err != nil {
		return nil, err
	}
	if err = conn.handshakeServer(context.Background(), l.l); err != nil {
		return nil, err
	}
	return conn, nil
}

func (l *Listener) Addr() netip.AddrPort { return l.raw.Addr() }

func (l *Listener) Close() error {
	err := errorx.Join(
		l.l.Close(),
		l.raw.Close(),
	)
	return err
}
