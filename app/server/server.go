//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/sconn"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
)

type Config struct {
	Sconn sconn.Config

	MTU uint16
}

type Server struct {
	cfg *Config

	l relraw.Listener

	Addr netip.AddrPort

	ap *PortAdapter
}

func ListenAndServe(ctx context.Context, addr string, cfg *Config) (err error) {
	var addrPort netip.AddrPort
	if a, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return err
	} else {
		if a.Port == 0 {
			a.Port = itun.DefaultPort
		}

		addr, ok := netip.AddrFromSlice(a.IP)
		if !ok {
			if len(a.IP) == 0 {
				addr = relraw.LocalAddr()
			} else {
				return fmt.Errorf("invalid address %s", a.IP)
			}
		} else if addr.Is4In6() {
			addr = netip.AddrFrom4(addr.As4())
		}
		addrPort = netip.AddrPortFrom(addr, uint16(a.Port))
	}

	var s = &Server{
		cfg:  cfg,
		Addr: addrPort,
		ap:   NewPortAdapter(addrPort.Addr()),
	}
	s.l, err = bpf.Listen(addrPort)
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
