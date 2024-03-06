//go:build linux
// +build linux

package server

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/app/server/proxyer"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
)

type Config struct {
	config.Config

	MTU                 uint16
	ProxyerIdeleTimeout time.Duration
}

type Server struct {
	cfg *Config

	l relraw.Listener

	Addr netip.AddrPort

	ap *adapter.Ports

	stack       ustack.Ustack
	ctrListener *gonet.TCPListener
}

func ListenAndServe(ctx context.Context, l relraw.Listener, cfg *Config) (err error) {
	var s = &Server{
		cfg:  cfg,
		l:    l,
		Addr: l.Addr(),
		ap:   adapter.NewPorts(l.Addr().Addr()),
	}
	s.stack, err = ustack.NewUstack(l.Addr(), int(cfg.MTU))
	if err != nil {
		return err
	}
	s.ctrListener, err = gonet.ListenTCP(s.stack, l.Addr(), header.IPv4ProtocolNumber)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rconn, err := s.l.Accept()
		if err != nil {
			return err
		}

		go proxyer.Proxy(ctx, s, itun.WrapRawConn(rconn, cfg.MTU))
	}
}

func (s *Server) Config() config.Config       { return s.Config() } // clone
func (s *Server) PortAdapter() *adapter.Ports { return s.ap }
func (s *Server) AcceptBy(ctx context.Context, src netip.AddrPort) (net.Conn, error) {
	return s.ctrListener.AcceptBy(ctx, src)
}
func (s *Server) Stack() ustack.Ustack { return s.stack }
