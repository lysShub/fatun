package session

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Client interface {
	Logger() *slog.Logger

	Uplink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

type Session struct {
	mgr    *SessionMgr
	client Client
	id     session.ID

	srvCtx    context.Context
	srvCancel context.CancelFunc
	capture   capture.Session

	closed   atomic.Bool
	closeErr error
	cnt      atomic.Uint32
}

func newSession(
	mgr *SessionMgr, client Client,
	id session.ID, session capture.Session,
) (s *Session, err error) {
	s = &Session{
		mgr:    mgr,
		client: client,

		capture: session,
		id:      id,
	}

	go s.uplinkService()
	s.keepalive()
	return s, nil
}

func (s *Session) close(cause error) {
	if s.closed.CompareAndSwap(false, true) {
		s.mgr.del(s.id)

		s.srvCancel()

		s.closeErr = errorx.Join(
			cause,
			s.capture.Close(),
		)
	}
}

func (s *Session) uplinkService() {
	var mtu = s.client.MTU()
	pkt := relraw.NewPacket(0, mtu)

	for {
		pkt.Sets(0, mtu)
		s.cnt.Add(1)

		err := s.capture.Capture(s.srvCtx, pkt)
		if err != nil {
			s.close(err)
			return
		}

		// todo: reset tcp mss

		err = s.client.Uplink(pkt, session.ID(s.id))
		if err != nil {
			s.close(err)
			return
		}
	}
}

func (s *Session) Inject(pkt *relraw.Packet) error {
	err := s.capture.Inject(pkt)
	if err != nil {
		s.close(err)
	}

	s.cnt.Add(1)
	return err
}

func (s *Session) keepalive() {
	const magic uint32 = 0x23df83a0
	switch s.cnt.Load() {
	case magic:
		s.close(itun.KeepaliveExceeded)
	default:
		s.cnt.Store(magic)
		time.AfterFunc(time.Minute, s.keepalive) // todo: from config
	}
}
