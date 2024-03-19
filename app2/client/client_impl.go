package client

import (
	"context"
	"log/slog"

	cs "github.com/lysShub/itun/app2/client/session"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/rsocket"
)

type sessionIpml Client

type sessionIpmlPtr = *sessionIpml

var _ cs.Client = sessionIpmlPtr(nil)

func (s *sessionIpml) Uplink(pkt *rsocket.Packet, id session.ID) error {
	return (*Client)(s).uplink(pkt, id)
}
func (s *sessionIpml) MTU() int                 { return s.raw.MTU() }
func (s *sessionIpml) Context() context.Context { return s.ctx }
func (s *sessionIpml) Del(id session.ID, cause error) error {
	return s.sessionMgr.Del(id, cause)
}
func (s *sessionIpml) Logger() *slog.Logger { return s.logger }
