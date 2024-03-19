//go:build linux
// +build linux

package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app2/server/adapter"
	"github.com/lysShub/itun/app2/server/proxyer"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/rsocket"
)

func ListenAndServe(ctx context.Context, l rsocket.Listener, cfg *Config) error {
	s, err := NewServer(l, cfg)
	if err != nil {
		return err
	}

	return s.Serve(ctx)
}

type Config struct {
	config.Config

	MTU                 uint16
	ProxyerIdeleTimeout time.Duration
}

type Server struct {
	cfg    *Config
	logger *slog.Logger

	l    rsocket.Listener
	Addr netip.AddrPort

	ap *adapter.Ports

	stack       ustack.Ustack
	ctrListener *gonet.TCPListener
}

func NewServer(l rsocket.Listener, cfg *Config) (*Server, error) {
	log := cfg.Log
	if log == nil {
		log = slog.NewJSONHandler(os.Stdout, nil)
	}
	log = log.WithGroup("server").WithAttrs([]slog.Attr{
		{Key: "addr", Value: slog.StringValue(l.Addr().String())},
	})

	var s = &Server{
		cfg:    cfg,
		logger: slog.New(log),
		l:      l,
		Addr:   l.Addr(),
		ap:     adapter.NewPorts(l.Addr().Addr()),
	}

	var err error
	s.stack, err = ustack.NewUstack(
		link.NewList(16, int(cfg.MTU)),
		l.Addr().Addr(),
	)
	if err != nil {
		return nil, err
	}
	s.ctrListener, err = gonet.ListenTCP(s.stack, l.Addr(), header.IPv4ProtocolNumber)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("starting")
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

		go proxyer.Proxy(ctx, s, itun.WrapRawConn(rconn, s.cfg.MTU))
	}
}

func (s *Server) Config() config.Config       { return s.cfg.Config } // clone
func (s *Server) PortAdapter() *adapter.Ports { return s.ap }
func (s *Server) AcceptBy(ctx context.Context, src netip.AddrPort) (net.Conn, error) {
	return s.ctrListener.AcceptBy(ctx, src)
}
func (s *Server) Stack() ustack.Ustack { return s.stack }
func (s *Server) Logger() *slog.Logger { return s.logger }
