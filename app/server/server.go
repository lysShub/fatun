//go:build linux
// +build linux

package server

import (
	"context"
	"net/netip"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
)

type Config struct {
	config.Config
	Key crypto.SecretKey

	MTU                 uint16
	TCPHandshakeTimeout time.Duration
	InitCfgTimeout      time.Duration
	ProxyerIdeleTimeout time.Duration
}

type Server struct {
	cfg *Config

	l relraw.Listener

	Addr netip.AddrPort

	ap *PortAdapter

	st          *ustack.Ustack
	ctrListener *gonet.TCPListener
}

func ListenAndServe(ctx context.Context, l relraw.Listener, cfg *Config) (err error) {
	var s = &Server{
		cfg:  cfg,
		l:    l,
		Addr: l.Addr(),
		ap:   NewPortAdapter(l.Addr().Addr()),
	}
	s.st, err = ustack.NewUstack(l.Addr(), int(cfg.MTU))
	if err != nil {
		return err
	}
	s.ctrListener, err = gonet.ListenTCP(s.st, l.Addr(), header.IPv4ProtocolNumber)
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

		go Proxy(ctx, s, itun.WrapRawConn(rconn, cfg.MTU))
	}
}
