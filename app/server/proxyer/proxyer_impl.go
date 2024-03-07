package proxyer

import (
	"context"
	"log/slog"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app"
	ss "github.com/lysShub/itun/app/server/proxyer/session"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type sessionImpl Proxyer

type proxyerImplPtr = *sessionImpl

var _ ss.Proxyer = (proxyerImplPtr)(nil)

func (s *sessionImpl) Downlink(pkt *relraw.Packet, id session.ID) error {
	return (*Proxyer)(s).downlink(pkt, id)
}
func (s *sessionImpl) MTU() int                                   { return s.raw.MTU() }
func (s *sessionImpl) Context() context.Context                   { return s.ctx }
func (s *sessionImpl) Del(id session.ID, cause error) (err error) { return s.sessionMgr.Del(id, cause) }
func (s *sessionImpl) Error(msg string, args ...any)              { s.logger.Error(msg, args...) }

type controlImpl Proxyer

type controlImplPtr = *controlImpl

var _ control.Handler = (controlImplPtr)(nil)

func (c *controlImpl) IPv6() bool {
	return true
}
func (c *controlImpl) EndConfig() {
	select {
	case <-c.endConfigNotify:
	default:
		close(c.endConfigNotify)
	}
}
func (c *controlImpl) AddTCP(addr netip.AddrPort) (session.ID, error) {
	s, err := c.sessionMgr.Add(
		proxyerImplPtr(c),
		session.Session{
			Src:   c.raw.RemoteAddrPort(),
			Proto: itun.TCP,
			Dst:   addr,
		},
	)
	if err != nil {
		c.logger.Error(err.Error(), app.TraceAttr(err))
		return 0, err
	} else {
		c.logger.LogAttrs(context.Background(), slog.LevelInfo, "add tcp session",
			slog.Attr{Key: "dst", Value: slog.StringValue(addr.String())},
			slog.Attr{Key: "id", Value: slog.IntValue(int(s.ID()))},
		)
	}
	return s.ID(), nil
}
func (c *controlImpl) DelTCP(id session.ID) error {
	return c.sessionMgr.Del(id, nil)
}
func (c *controlImpl) AddUDP(addr netip.AddrPort) (session.ID, error) {
	s, err := c.sessionMgr.Add(
		proxyerImplPtr(c),
		session.Session{
			Proto: itun.UDP,
			Dst:   addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (c *controlImpl) DelUDP(id session.ID) error {
	return c.sessionMgr.Del(id, nil)
}
func (c *controlImpl) PackLoss() float32 {
	return 0
}
func (c *controlImpl) Ping() {}
