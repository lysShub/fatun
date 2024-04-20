package session

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/app/client/capture"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"
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
	if s.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if s.mgr != nil {
			s.mgr.del(s.id)
		}

		s.srvCancel()

		if s.capture != nil {
			if err := s.capture.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			s.closeErr.Store(&cause)
			s.client.Logger().Warn("session close", cause.Error(), errorx.TraceAttr(cause))
		} else {
			s.client.Logger().Info("session close")
		}
		return cause
	}
	return *s.closeErr.Load()
}

func (s *Session) uplinkService() error {
	var (
		pkt = packet.Make(64, s.client.MTU())
	)

	for {
		s.cnt.Add(1)

		err := s.capture.Capture(s.srvCtx, pkt.SetHead(64))
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
		s.close(errors.WithStack(app.KeepaliveExceeded))
	default:
		s.cnt.Store(magic)
		time.AfterFunc(time.Minute, s.keepalive) // todo: from config
	}
}
