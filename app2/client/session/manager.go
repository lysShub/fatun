package session

import (
	"context"
	"sync"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app2/client/capture"
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

func (m *SessionMgr) Add(client Client, s capture.Session, id session.ID) error {
	if _, err := m.Get(id); err == nil {
		return session.ErrExistID(id)
	}

	session, err := newSession(m, client, id, s)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[id] = session
	return nil
}

func (m *SessionMgr) Get(id session.ID) (s *Session, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s = m.sessions[id]
	if s == nil {
		err = session.ErrInvalidID(id)
	}
	return s, err
}

func (m *SessionMgr) Del(id session.ID, cause error) error {
	if s, err := m.Get(id); err != nil {
		return err
	} else {
		return m.del(s, cause)
	}
}

func (m *SessionMgr) del(s *Session, cause error) error {
	m.mu.Lock()
	delete(m.sessions, s.id)
	m.mu.Unlock()

	return s.close(cause)
}

func (m *SessionMgr) keepalive() {
	var ss = make([]*Session, 0, 8)

	for {
		ss = ss[:0]

		select {
		case <-m.ticker.C:
			m.mu.RLock()
			for _, e := range m.sessions {
				if e.tick() {
					ss = append(ss, e)
				}
			}
			m.mu.RUnlock()
		case <-m.closeCh:
			return
		}

		for _, e := range ss {
			m.Del(e.id, itun.KeepaliveExceeded)
		}
	}
}

func (m *SessionMgr) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ticker.Stop()
	close(m.closeCh)

	for _, e := range m.sessions {
		e.close(context.Canceled)
	}
	clear(m.sessions)
	return nil
}
