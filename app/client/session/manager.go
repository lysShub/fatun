package session

import (
	"context"
	"sync"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
)

type SessionMgr struct {
	ticker  *time.Ticker
	closeCh chan struct{}

	mu       sync.RWMutex
	sessions map[session.ID]*Session
}

func NewSessionMgr() *SessionMgr {
	var sm = &SessionMgr{
		ticker:  time.NewTicker(time.Minute), // todo: from config
		closeCh: make(chan struct{}),

		sessions: make(map[session.ID]*Session, 16),
	}
	go sm.keepalive()
	return sm
}

func (sm *SessionMgr) Add(clientCtx context.Context, up Uplink, s session.Session, id session.ID) error {
	if _, err := sm.Get(id); err != nil {
		return err
	}

	session, err := newSession(clientCtx, up, id, s)
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
