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
	"github.com/lysShub/sockit/packet"
)

type Client interface {
	Logger() *slog.Logger

	Uplink(pkt *packet.Packet, id session.ID) error
	MTU() int
}

type Session struct {
	mgr    *SessionMgr
	client Client
	id     session.ID

	srvCtx    context.Context
	srvCancel context.CancelFunc
	capture   capture.Session

	closeErr atomic.Pointer[error]
	cnt      atomic.Uint32
}

func newSession(
	mgr *SessionMgr, client Client,
	id session.ID, session capture.Session,
) (s *Session, err error) {
	s = &Session{
		mgr:    mgr,
		client: client,
		id:     id,

		capture: session,
	}
	s.srvCtx, s.srvCancel = context.WithCancel(context.Background())

	go s.uplinkService()
	s.keepalive()
	return s, nil
}

func (s *Session) close(cause error) error {
	if cause == nil {
		return *s.closeErr.Load()
	}

	if s.closeErr.CompareAndSwap(nil, &cause) {
		s.mgr.del(s.id)

		s.srvCancel()

		err := errorx.Join(
			cause,
			s.capture.Close(),
		)

		s.client.Logger().Info("session close")
		s.closeErr.Store(&err)
		return err
	} else {
		return *s.closeErr.Load()
	}
}

func (s *Session) uplinkService() error {
	var mtu = s.client.MTU()
	pkt := packet.Make(0, mtu)

	for {
		pkt.Sets(0, mtu)
		s.cnt.Add(1)

		err := s.capture.Capture(s.srvCtx, pkt)
		if err != nil {
			return s.close(err)
		}

		// todo: reset tcp mss

		err = s.client.Uplink(pkt, session.ID(s.id))
		if err != nil {
			return s.close(err)
		}
	}
}

func (s *Session) Inject(pkt *packet.Packet) error {
	err := s.capture.Inject(pkt)
	if err != nil {
		return s.close(err)
	}

	s.cnt.Add(1)
	return nil
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
