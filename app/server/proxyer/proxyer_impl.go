package proxyer

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	"github.com/lysShub/itun/app/server/adapter"
	ss "github.com/lysShub/itun/app/server/proxyer/session"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
)

type sessionImpl Proxyer

type proxyerImplPtr = *sessionImpl

var _ ss.Proxyer = (proxyerImplPtr)(nil)

func (s *sessionImpl) MTU() int                 { return s.cfg.MTU }
func (s *sessionImpl) Logger() *slog.Logger     { return s.logger }
func (s *sessionImpl) Addr() netip.AddrPort     { return s.conn.LocalAddr() }
func (s *sessionImpl) Adapter() *adapter.Ports  { return s.srv.Adapter() }
func (s *sessionImpl) Keepalive() time.Duration { return time.Minute } // todo: from config
func (s *sessionImpl) Downlink(pkt *packet.Packet, id session.ID) error {
	return (*Proxyer)(s).downlink(pkt, id)
}

type controlImpl Proxyer

type controlImplPtr = *controlImpl

var _ control.Handler = (controlImplPtr)(nil)

func (c *controlImpl) IPv6() bool {
	return true
}
func (c *controlImpl) InitConfig(cfg *control.Config) error {
	c.logger.Info("start")
	return nil
}
func (c *controlImpl) AddSession(sess session.Session) (session.ID, error) {
	s, err := c.sessionMgr.Add(sess)
	if err != nil {
		c.logger.Error(err.Error(), errorx.TraceAttr(err))
		return 0, err
	} else {
		c.logger.LogAttrs(context.Background(), slog.LevelInfo, "add tcp session",
			slog.Attr{Key: "localAddr", Value: slog.StringValue(s.LocalAddr().String())},
			slog.Attr{Key: "dst", Value: slog.StringValue(sess.Dst().String())},
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
