package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Uplink interface {
	uplink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

type SessionMgr struct {
	client *Client

	ticker  *time.Ticker
	closeCh chan struct{}

	mu       sync.RWMutex
	sessions map[session.ID]*Session
}

func NewSessionMgr(c *Client) *SessionMgr {
	var sm = &SessionMgr{
		client:  c,
		ticker:  time.NewTicker(time.Minute), // todo: from config
		closeCh: make(chan struct{}),

		sessions: make(map[session.ID]*Session, 16),
	}
	go sm.keepalive()
	return sm
}

func (sm *SessionMgr) Add(s session.Session, id session.ID) error {
	if _, err := sm.Get(id); err != nil {
		return err
	}

	session, err := newSession(sm.client.ctx, sm.client, id, s)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[id] = session
	return nil
}

func (sm *SessionMgr) Get(id session.ID) (s *Session, err error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s = sm.sessions[id]
	if s == nil {
		err = session.ErrInvalidID(id)
	}
	return s, err
}

func (sm *SessionMgr) Del(id session.ID, cause error) error {
	if s, err := sm.Get(id); err != nil {
		return err
	} else {
		return sm.del(s, cause)
	}
}

func (sm *SessionMgr) del(s *Session, cause error) error {
	err := s.close(cause)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, s.id)
	return err
}

func (sm *SessionMgr) keepalive() {
	var ss = make([]*Session, 0, 8)

	for {
		ss = ss[:0]

		select {
		case <-sm.ticker.C:
			sm.mu.RLock()
			for _, e := range sm.sessions {
				if e.tick() {
					ss = append(ss, e)
				}
			}
			sm.mu.RUnlock()
		case <-sm.closeCh:
			return
		}

		for _, e := range ss {
			sm.Del(e.id, itun.KeepaliveExceeded)
		}
	}
}

func (sm *SessionMgr) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ticker.Stop()
	close(sm.closeCh)

	for _, e := range sm.sessions {
		e.close(context.Canceled)
	}
	clear(sm.sessions)
	return nil
}

type Session struct {
	ctx    cctx.CancelCtx
	closed atomic.Bool

	session session.Session
	id      session.ID

	capture Capture

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
	s.capture, err = NewCapture(session)
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

		up.uplink(seg, session.ID(s.id))
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
