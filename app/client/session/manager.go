package session

import (
	"sync"

	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
)

type SessionMgr struct {
	closeCh chan struct{}

	mu       sync.RWMutex
	sessions map[session.ID]Session
}

func NewSessionMgr() *SessionMgr {
	var sm = &SessionMgr{
		sessions: make(map[session.ID]Session, 16),
	}
	return sm
}

func (sm *SessionMgr) Close() error {
	var ss []Session
	sm.mu.Lock()
	for _, e := range sm.sessions {
		ss = append(ss, e)
	}
	clear(sm.sessions)
	sm.mu.Unlock()

	for _, e := range ss {
		e.Close()
	}
	return nil
}

func (sm *SessionMgr) Add(client Client, id session.ID, firstIP []byte) error {
	if _, err := sm.Get(id); err == nil {
		return errorx.WrapTemp(session.ErrExistID(id))
	}

	sess, err := newSession(client, id, firstIP)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[id] = sess
	return nil
}

func (sm *SessionMgr) Get(id session.ID) (s Session, err error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s = sm.sessions[id]
	if s == nil {
		err = session.ErrInvalidID(id)
	}
	return s, err
}

func (sm *SessionMgr) Del(id session.ID) {
	sm.mu.Lock()
	delete(sm.sessions, id)
	sm.mu.Unlock()
}
