package proxyer

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	ss "github.com/lysShub/itun/app/server/proxyer/session"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type sessionImpl Proxyer

type proxyerImplPtr = *sessionImpl

var _ ss.Proxyer = (proxyerImplPtr)(nil)

func (s *sessionImpl) Downlink(pkt *relraw.Packet, id session.ID) error {
	return (*Proxyer)(s).downlink(pkt, id)
}
func (s *sessionImpl) MTU() int                 { return s.cfg.MTU }
func (s *sessionImpl) Logger() *slog.Logger     { return s.logger }
func (s *sessionImpl) Addr() netip.AddrPort     { return s.conn.LocalAddr() }
func (s *sessionImpl) Keepalive() time.Duration { return time.Minute } // todo: config

type controlImpl Proxyer

type controlImplPtr = *controlImpl

var _ control.Handler = (controlImplPtr)(nil)

func (c *controlImpl) IPv6() bool {
	return true
}
func (c *controlImpl) EndConfig() {}
func (c *controlImpl) AddSession(sess session.Session) (session.ID, error) {
	s, err := c.sessionMgr.Add(sess)
	if err != nil {
		c.logger.Error(err.Error(), errorx.TraceAttr(err))
		return 0, err
	} else {
		c.logger.LogAttrs(context.Background(), slog.LevelInfo, "add tcp session",
			slog.Attr{Key: "localAddr", Value: slog.StringValue(s.LocalAddr().String())},
			slog.Attr{Key: "dst", Value: slog.StringValue(sess.Dst.String())},
			slog.Attr{Key: "id", Value: slog.IntValue(int(s.ID()))},
		)
	}
	return s.ID(), nil
}
func (c *controlImpl) DelSession(id session.ID) error {
	return c.sessionMgr.Del(id, nil)
}
func (c *controlImpl) PackLoss() float32 {
	return 0
}
func (c *controlImpl) Ping() {

}
