package client

import (
	"context"

	"github.com/lysShub/itun/app/client/capture"
	cs "github.com/lysShub/itun/app/client/session"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type sessionIpml Client

type sessionIpmlPtr = *sessionIpml

var _ cs.Client = sessionIpmlPtr(nil)

func (s *sessionIpml) Uplink(pkt *relraw.Packet, id session.ID) error {
	return (*Client)(s).uplink(pkt, id)
}
func (s *sessionIpml) MTU() int                 { return s.raw.MTU() }
func (s *sessionIpml) Context() context.Context { return s.ctx }
func (s *sessionIpml) Del(id session.ID, cause error) error {
	return s.sessionMgr.Del(id, cause)
}
func (s *sessionIpml) Error(msg string, args ...any) {
	s.logger.Error(msg, args...)
}

func (s *sessionIpml) DelSession(sess capture.Session) {
	s.capture.Del(sess.Session())
}
