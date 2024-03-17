package server

import (
	"context"
	"log/slog"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/app/server/proxyer"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func ListenAndServe(ctx context.Context, l *sconn.Listener, cfg *app.Config) error {
	s, err := NewServer(l, cfg)
	if err != nil {
		return err
	}

	return s.Serve(ctx)
}

type Server struct {
	cfg    *app.Config
	logger *slog.Logger

	raw *sconn.Listener

	ap *adapter.Ports

	stack ustack.Ustack
	l     *gonet.TCPListener
}

func NewServer(l *sconn.Listener, cfg *app.Config) (*Server, error) {
	var s = &Server{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("server").WithAttrs([]slog.Attr{
			{Key: "addr", Value: slog.StringValue(l.Addr().String())},
		})),
		raw: l,
		ap:  adapter.NewPorts(l.Addr().Addr()),
	}

	var err error
	s.stack, err = ustack.NewUstack(link.NewList(64, cfg.MTU), l.Addr().Addr())
	if err != nil {
		return nil, err
	}
	s.l, err = gonet.ListenTCP(s.stack, l.Addr(), header.IPv4ProtocolNumber)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("start")

	for {
		conn, err := s.raw.AcceptCtx(ctx)
		if err != nil {
			return err
		}

		go proxyer.Proxy(ctx, proxyerImplPtr(s), conn)
	}

}
