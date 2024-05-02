package sconn

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/rawsock"
	"github.com/pkg/errors"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Listener struct {
	cfg *Config
	raw rawsock.Listener

	stack ustack.Ustack
	l     *gonet.TCPListener

	closeErr atomic.Pointer[error]
}

func NewListener(l rawsock.Listener, cfg *Config) (*Listener, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}
	var li = &Listener{cfg: cfg, raw: l}

	var err error
	if li.stack, err = ustack.NewUstack(
		link.NewList(64, cfg.MTU), l.Addr().Addr(),
	); err != nil {
		return nil, li.close(err)
	}
	// li.stack = utest.MustWrapPcap("server.pcap", li.stack)

	if li.l, err = gonet.ListenTCP(
		li.stack, l.Addr(),
		header.IPv4ProtocolNumber,
	); err != nil {
		return nil, li.close(err)
	}

	return li, nil
}

func (l *Listener) close(cause error) error {
	if l.closeErr.CompareAndSwap(nil, &net.ErrClosed) {
		if l.l != nil {
			if err := l.l.Close(); err != nil {
				cause = errors.WithStack(err)
			}
		}
		if l.stack != nil {
			if err := l.stack.Close(); err != nil {
				cause = err
			}
		}
		if l.raw != nil {
			if err := l.raw.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			l.closeErr.Store(&cause)
		}
		return cause
	}
	return *l.closeErr.Load()
}

func (l *Listener) Accept() (*Conn, error) {
	return l.AcceptCtx(context.Background())
}

func (l *Listener) AcceptCtx(ctx context.Context) (*Conn, error) {
	raw, err := l.raw.Accept() // todo: raw support context
	if err != nil {
		return nil, err
	}

	ep, err := l.stack.LinkEndpoint(l.Addr().Port(), raw.RemoteAddr())
	if err != nil {
		return nil, err
	}
	conn, err := newConn(raw, ep, server, l.cfg)
	if err != nil {
		return nil, err
	}
	conn.factory = &serverFactory{l: l.l, remote: conn.RemoteAddr()}
	return conn, nil
}

func (l *Listener) Addr() netip.AddrPort { return l.raw.Addr() }

func (l *Listener) Close() error { return l.close(nil) }
