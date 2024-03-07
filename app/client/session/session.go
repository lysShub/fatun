package session

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Client interface {
	Context() context.Context
	Del(id session.ID, cause error) error
	Error(msg string, args ...any)

	Uplink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

type Session struct {
	ctx    cctx.CancelCtx
	client Client
	closed atomic.Bool

	session session.Session
	id      session.ID

	capture capture.Capture

	cnt atomic.Uint32
}

func newSession(
	client Client,
	id session.ID, session session.Session,
) (s *Session, err error) {
	s = &Session{
		ctx:    cctx.WithContext(client.Context()),
		client: client,

		session: session,
		id:      id,
	}
	s.capture, err = capture.NewCapture(session)
	if err != nil {
		return nil, err
	}

	go s.uplinkService()
	return s, nil
}

func (s *Session) uplinkService() {
	var mtu = s.client.MTU()
	seg := relraw.NewPacket(0, mtu)

	for {
		seg.Sets(0, mtu)
		s.cnt.Add(1)

		err := s.capture.Capture(s.ctx, seg)
		if err != nil {
			s.client.Del(s.id, err)
			return
		}

		// todo: reset tcp mss

		err = s.client.Uplink(seg, session.ID(s.id))
		if err != nil {
			s.client.Del(s.id, err)
			return
		}
	}
}

func (s *Session) Inject(seg *relraw.Packet) {
	err := s.capture.Inject(seg)
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

		err := errors.Join(
			s.ctx.Err(),
			s.capture.Close(),
		)

		s.client.Error(err.Error(), app.TraceAttr(err))
		return err
	}
	return nil
}
