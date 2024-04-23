//go:build windows
// +build windows

package client

import (
	"context"
	"log/slog"
	"slices"

	cs "github.com/lysShub/fatun/app/client/session"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"
)

type sessionImpl Client

type sessionImplPtr = *sessionImpl

var _ cs.Client = (sessionImplPtr)(nil)

func (s *sessionImpl) Logger() *slog.Logger { return s.logger }
func (s *sessionImpl) Uplink(pkt *packet.Packet, id session.ID) error {
	return (*Client)(s).uplink(context.Background(), pkt, id)
}
func (s *sessionImpl) MTU() int              { return s.cfg.MTU }
func (s *sessionImpl) DivertPriority() int16 { return s.divertPriority - 1 }
func (s *sessionImpl) Release(id session.ID) { s.sessMgr.Del(id) }

type captureImpl Client
type captureImplPtr = *captureImpl

func (c *captureImpl) raw() *Client          { return ((*Client)(c)) }
func (c *captureImpl) Logger() *slog.Logger  { return c.logger }
func (c *captureImpl) MTU() int              { return c.cfg.MTU }
func (c *captureImpl) DivertPriority() int16 { return c.divertPriority - 2 } // capture should read firstly
func (c *captureImpl) Hit(ip []byte) bool {
	hit, err := c.hiter.Hit(ip)
	if err != nil {
		if errorx.Temporary(err) {
			c.logger.Warn(err.Error(), errorx.TraceAttr(err))
		} else {
			c.raw().close(err)
		}
	} else if hit {
		// add session
		id := session.FromIP(ip)
		if id == c.self {
			c.raw().close(errors.Errorf("can't proxy self %s", c.self.String()))
		}

		resp, err := c.ctr.AddSession(c.srvCtx, id) // todo: add timeout
		if err != nil {
			c.raw().close(err)
		} else if resp.Err != "" {
			err = errors.New(resp.Err)
			c.logger.Warn(err.Error(), errorx.TraceAttr(err))
		} else {
			err = c.sessMgr.Add(sessionImplPtr(c.raw()), resp.ID, slices.Clone(ip))
			if err != nil {
				if errorx.Temporary(err) {
					c.logger.Warn(err.Error(), errorx.TraceAttr(err))
				} else {
					c.raw().close(err)
				}
			}
		}
		c.logger.LogAttrs(c.srvCtx, slog.LevelInfo, "add session", slog.Attr{
			Key: "session", Value: slog.StringValue(id.String()),
		})
	}
	return hit
}
