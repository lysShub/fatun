package session

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/lysShub/itun/app2/client/capture"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Client interface {
	Context() context.Context
	Del(id session.ID, cause error) error
	Logger() *slog.Logger

	Uplink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

type Session struct {
	ctx    cctx.CancelCtx
	mgr    *SessionMgr
	client Client
	id     session.ID
	closed atomic.Bool

	capture capture.Session

	cnt atomic.Uint32
}

func newSession(
	mgr *SessionMgr, client Client,
	id session.ID, session capture.Session,
) (s *Session, err error) {
	s = &Session{
		ctx:    cctx.WithContext(client.Context()),
		mgr:    mgr,
		client: client,

		capture: session,
		id:      id,
	}

	go s.uplinkService()
	return s, nil
}

func (s *Session) uplinkService() {
	var mtu = s.client.MTU()
	pkt := relraw.NewPacket(0, mtu)

	for {
		pkt.Sets(0, mtu)
		s.cnt.Add(1)

		err := s.capture.Capture(s.ctx, pkt)
		if err != nil {
			s.client.Del(s.id, err)
			return
		}

		// todo: reset tcp mss

		err = s.client.Uplink(pkt, session.ID(s.id))
		if err != nil {
			s.client.Del(s.id, err)
			return
		}
	}
}

func (s *Session) Inject(pkt *relraw.Packet) {
	err := s.capture.Inject(pkt)
	if err != nil {
		s.client.Del(s.id, err)
	}

	s.cnt.Add(1)
}

func (s *Session) tick() bool {
	const magic uint32 = 0x23df83a0
	if s.cnt.Load() == magic {
		return true
	} else {
		s.cnt.Store(magic)
		return false
	}
}

func (s *Session) close(cause error) error {
	if s.closed.CompareAndSwap(false, true) {
		s.ctx.Cancel(cause)

		s.mgr.del(s, cause)

		if errorx.Temporary(cause) {
			s.client.Logger().LogAttrs(
				context.Background(), slog.LevelWarn, cause.Error(),
				slog.Attr{Key: "session", Value: slog.StringValue(s.capture.String())},
			)
		} else {
			s.client.Logger().Error(cause.Error(), errorx.TraceAttr(cause))
		}
		return cause
	}
	return nil
}
