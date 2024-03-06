package session

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Uplink interface {
	Uplink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

type Session struct {
	ctx    cctx.CancelCtx
	closed atomic.Bool

	session session.Session
	id      session.ID

	capture capture.Capture

	cnt atomic.Uint32
}

func newSession(
	clientCtx context.Context, up Uplink,
	id session.ID, session session.Session,
) (s *Session, err error) {
	s = &Session{
		ctx:     cctx.WithContext(clientCtx),
		session: session,
		id:      id,
	}
	s.capture, err = capture.NewCapture(session)
	if err != nil {
		return nil, err
	}

	go s.uplinkService(up)
	return s, nil
}

func (s *Session) uplinkService(up Uplink) {
	var mtu = up.MTU()
	seg := relraw.NewPacket(0, mtu)

	for {
		seg.Sets(0, mtu)
		if err := s.capture.Capture(s.ctx, seg); err != nil {
			s.ctx.Cancel(err)
			return
		}

		// todo: reset tcp mss

		up.Uplink(seg, session.ID(s.id))
		s.cnt.Add(1)
	}
}

func (s *Session) Inject(seg *relraw.Packet) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}
	s.cnt.Add(1)

	err := s.capture.Inject(seg)
	return err
}

func (s *Session) tick() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
	}

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

		err := s.ctx.Err()
		err = errors.Join(err,
			s.capture.Close(),
		)

		fmt.Println(err) // tood: log
		return err
	}
	return nil
}
