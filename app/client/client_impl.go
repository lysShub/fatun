package client

import (
	"context"
	"log/slog"

	cs "github.com/lysShub/fatun/app/client/session"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
)

type sessionImpl Client

type sessionImplPtr = *sessionImpl

var _ cs.Client = (sessionImplPtr)(nil)

func (s *sessionImpl) Logger() *slog.Logger {
	return s.logger
}
func (s *sessionImpl) Uplink(pkt *packet.Packet, id session.ID) error {
	return (*Client)(s).uplink(context.Background(), pkt, id)
}
func (s *sessionImpl) MTU() int { return s.cfg.MTU }
