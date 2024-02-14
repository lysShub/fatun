//go:build linux
// +build linux

package server

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/config"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
)

type Server struct {
	cfg *config.Server

	l relraw.Listener

	Addr netip.AddrPort

	ap *PortAdapter
}

func ListenAndServer(ctx context.Context, addr string, cfg *config.Server) error {
	a, err := netip.ParseAddrPort(addr)
	if err != nil {
		return err
	}
	var s = &Server{
		cfg:  cfg,
		Addr: a,
		ap:   NewPortAdapter(a.Addr()),
	}

	s.l, err = bpf.Listen(a)
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

		go Handle(ctx, s, itun.WrapRawConn(rconn))
	}
}
