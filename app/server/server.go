package server

import (
	"context"
	"log/slog"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/app/server/proxyer"
	"github.com/lysShub/itun/sconn"
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

	l *sconn.Listener

	ap *adapter.Ports
}

func NewServer(l *sconn.Listener, cfg *app.Config) (*Server, error) {
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
