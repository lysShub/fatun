package sconn

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Listener struct {
	cfg *Config
	raw relraw.Listener

	stack ustack.Ustack
	l     *gonet.TCPListener
}

func NewListener(l relraw.Listener, cfg *Config) (*Listener, error) {
	var err error
	if err = cfg.init(); err != nil {
		return nil, err
	}

	var listener = &Listener{
		cfg: cfg,
		raw: l,
	}

	link := link.WrapNofin(link.NewList(64, cfg.MTU))
	listener.stack, err = ustack.NewUstack(link, l.Addr().Addr())
	if err != nil {
		return nil, err
	}

	listener.l, err = gonet.ListenTCP(listener.stack, l.Addr(), header.IPv4ProtocolNumber)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

func (l *Listener) Accept() (*Conn, error) {
	return l.AcceptCtx(context.Background())
}

func (l *Listener) AcceptCtx(ctx context.Context) (*Conn, error) {
	raw, err := l.raw.Accept() // todo: raw support context
	if err != nil {
		return nil, err
	}

	conn, err := newConn(raw, server, l.cfg)
	if err != nil {
		return nil, err
	}

	err = conn.handshakeAccept(context.Background(), l.stack, l.l)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (l *Listener) Addr() netip.Addr { return l.raw.Addr().Addr() }

func (l *Listener) Close() error {
	err := errorx.Join(
		l.l.Close(),
		l.raw.Close(),
	)
	return err
}
