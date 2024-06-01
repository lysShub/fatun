package conn

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Listener interface {
	Accept() (dgramConn Conn, err error)
	Addr() netip.AddrPort
	Close() error
}

type listener struct {
	config *Config
	peer   Peer
	laddr  netip.AddrPort

	l net.Listener

	stack           ustack.Ustack
	builtinListener *gonet.TCPListener

	closeErr errorx.CloseErr
}

func NewListen[P Peer](dgramConnlistener net.Listener, config *Config) (Listener, error) {
	var l = &listener{config: config, peer: *new(P), l: dgramConnlistener}
	l.laddr = netip.MustParseAddrPort(dgramConnlistener.Addr().String())
	var err error

	l.stack, err = ustack.NewUstack(link.NewList(128, 512), l.laddr.Addr()) // todo: fix mtu
	if err != nil {
		return nil, l.close(err)
	}
	if config.PcapBuiltinPath != "" {
		l.stack = ustack.MustWrapPcap(l.stack, config.PcapBuiltinPath)
	}
	l.builtinListener, err = gonet.ListenTCP(l.stack, l.laddr, header.IPv4ProtocolNumber)
	if err != nil {
		return nil, l.close(err)
	}

	return l, nil
}

func (l *listener) close(cause error) error {
	return l.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if l.builtinListener != nil {
			errs = append(errs, l.builtinListener.Close())
		}
		if l.stack != nil {
			errs = append(errs, l.stack.Close())
		}
		if l.l != nil {
			errs = append(errs, l.l.Close())
		}
		return errs
	})
}

func (l *listener) Accept() (Conn, error) {
	conn, err := l.l.Accept()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	raddr := netip.MustParseAddrPort(conn.RemoteAddr().String())
	ep, err := l.stack.LinkEndpoint(l.Addr().Port(), raddr)
	if err != nil {
		return nil, err
	}
	return newConn(conn, l.peer, server, ep, l.serverFactory, l.config)
}

func (l *listener) Addr() netip.AddrPort { return l.laddr }
func (l *listener) Close() error         { return l.close(nil) }

func (l *listener) serverFactory(ctx context.Context, remote netip.AddrPort) (*gonet.TCPConn, error) {
	return l.builtinListener.AcceptBy(ctx, remote)
}
