package udp

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/udp/audp"
	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Listener struct {
	addr   netip.AddrPort
	config *Config
	peer   conn.Peer

	l *audp.Listener

	stack           ustack.Ustack
	builtinListener *gonet.TCPListener

	closeErr errorx.CloseErr
}

var _ conn.Listener = (*Listener)(nil)

func Listen[P conn.Peer](server string, config *Config) (conn.Listener, error) {
	var l = &Listener{config: config, peer: *new(P)}
	var err error

	if l.addr, err = resolve(server, true); err != nil {
		return nil, l.close(err)
	}

	l.l, err = audp.Listen(
		&net.UDPAddr{IP: l.addr.Addr().AsSlice(), Port: int(l.addr.Port())}, config.MaxRecvBuff,
	)
	if err != nil {
		return nil, l.close(err)
	}

	l.stack, err = ustack.NewUstack(link.NewList(128, 512), l.addr.Addr()) // todo: fix mtu
	if err != nil {
		return nil, l.close(err)
	}
	if config.PcapBuiltinPath != "" {
		l.stack = ustack.MustWrapPcap(l.stack, config.PcapBuiltinPath)
	}
	l.builtinListener, err = gonet.ListenTCP(l.stack, l.addr, header.IPv4ProtocolNumber)
	if err != nil {
		return nil, l.close(err)
	}

	return l, nil
}

func (l *Listener) close(cause error) error {
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

func (l *Listener) Accept() (conn.Conn, error) {
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

func (l *Listener) Addr() netip.AddrPort { return l.addr }
func (l *Listener) Close() error         { return l.close(nil) }

func (l *Listener) serverFactory(ctx context.Context, remote netip.AddrPort) (*gonet.TCPConn, error) {
	return l.builtinListener.AcceptBy(ctx, remote)
}
