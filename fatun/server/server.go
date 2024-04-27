package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/adapter"
	"github.com/lysShub/fatun/fatun/server/proxyer"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/sockit/conn/tcp"
	"github.com/pkg/errors"
)

func ListenAndServe(ctx context.Context, addr string, cfg *fatun.Config) error {
	var laddr netip.AddrPort
	if addr, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return errors.WithStack(err)
	} else {
		ip := addr.IP
		if ip == nil {
			laddr = netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(addr.Port))
		} else if ip.To4() != nil {
			laddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ip.To4())), uint16(addr.Port))
		} else {
			laddr = netip.AddrPortFrom(netip.AddrFrom16([16]byte(ip.To16())), uint16(addr.Port))
		}
	}

	raw, err := tcp.Listen(laddr)
	if err != nil {
		return err
	}
	defer raw.Close()
	l, err := sconn.NewListener(raw, cfg.Config)
	if err != nil {
		return err
	}

	s, err := NewServer(l, cfg)
	if err != nil {
		return err
	}

	return s.Serve(ctx)
}

type Server struct {
	cfg    *fatun.Config
	logger *slog.Logger

	l *sconn.Listener

	ap *adapter.Ports
}

func NewServer(l *sconn.Listener, cfg *fatun.Config) (*Server, error) {
	var s = &Server{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("server").WithAttrs([]slog.Attr{
			{Key: "addr", Value: slog.StringValue(l.Addr().String())},
		})),
		l:  l,
		ap: adapter.NewPorts(l.Addr().Addr()),
	}

	return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("start")

	for {
		conn, err := s.l.AcceptCtx(ctx)
		if err != nil {
			return err
		}

		go proxyer.Proxy(ctx, proxyerImplPtr(s), conn)
	}
}
