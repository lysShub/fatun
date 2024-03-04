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

	st *ustack.Ustack
}

func ListenAndServe(ctx context.Context, l relraw.Listener, cfg *Config) (err error) {
	var s = &Server{
		cfg:  cfg,
		l:    l,
		Addr: l.Addr(),
		ap:   NewPortAdapter(l.Addr().Addr()),
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
