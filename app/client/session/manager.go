package session

import (
	"os"
	"sync"

	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/session"
)

type SessionMgr struct {
	closeCh chan struct{}

	mu       sync.RWMutex
	sessions map[session.ID]*Session
}

func NewSessionMgr() *SessionMgr {
	var sm = &SessionMgr{
		sessions: make(map[session.ID]*Session, 16),
	}
	return sm
}

func (sm *SessionMgr) Close() error {
	var ss []*Session
	sm.mu.Lock()
	for _, e := range sm.sessions {
		ss = append(ss, e)
	}
	clear(sm.sessions)
	sm.mu.Unlock()

	for _, e := range ss {
		e.close(os.ErrClosed)
	}
	return nil
}

func (sm *SessionMgr) Add(client Client, s capture.Session, id session.ID) error {
	if _, err := sm.Get(id); err == nil {
		return session.ErrExistID(id)
	}

	session, err := newSession(sm, client, id, s)
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

func (sm *SessionMgr) del(id session.ID) {
	if s, err := sm.Get(id); err == nil {
		sm.mu.Lock()
		delete(sm.sessions, s.id)
		sm.mu.Unlock()
	}
}
