package client

import (
	"context"
	"log/slog"

	cs "github.com/lysShub/itun/app/client/session"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type sessionImpl Client

type sessionImplPtr = *sessionImpl

var _ cs.Client = (sessionImplPtr)(nil)

func (s *sessionImpl) Logger() *slog.Logger {
	return s.logger
}
func (s *sessionImpl) Uplink(pkt *relraw.Packet, id session.ID) error {
	return (*Client)(s).uplink(context.Background(), pkt, id)
}
func (s *sessionImpl) MTU() int { return s.cfg.MTU }
