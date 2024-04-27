package session

import (
	"sync"

	"github.com/lysShub/fatun/session"
)

type SessionMgr struct {
	idsMu sync.RWMutex
	ids   map[session.ID]Session

	sessMu sync.RWMutex
	sesses map[session.Session]struct{}
}

func NewSessionMgr() *SessionMgr {
	var sm = &SessionMgr{
		ids:    make(map[session.ID]Session, 16),
		sesses: make(map[session.Session]struct{}, 16),
	}
	return sm
}

func (m *SessionMgr) Close() error {
	var ss []Session
	m.idsMu.Lock()
	for _, e := range m.ids {
		ss = append(ss, e)
	}
	clear(m.ids)
	m.idsMu.Unlock()

	m.sessMu.Lock()
	clear(m.sesses)
	m.sessMu.Unlock()

	for _, e := range ss {
		e.Close()
	}
	return nil
}

func (m *SessionMgr) Add(client Client, id session.ID, firstIP []byte) error {
	if m.Exist(firstIP) {
		return session.ErrExistID(id)
	}

	sess, err := newSession(client, id, firstIP)
	if err != nil {
		return err
	}

	m.idsMu.Lock()
	m.ids[id] = sess
	m.idsMu.Unlock()

	m.sessMu.Lock()
	m.sesses[session.FromIP(firstIP)] = struct{}{}
	m.sessMu.Unlock()
	return nil
}

func (m *SessionMgr) Get(id session.ID) (s Session, err error) {
	m.idsMu.RLock()
	defer m.idsMu.RUnlock()

	s = m.ids[id]
	if s == nil {
		err = session.ErrInvalidID(id)
	}
	return s, err
}

func (m *SessionMgr) Exist(firstIP []byte) bool {
	m.sessMu.RLock()
	_, has := m.sesses[session.FromIP(firstIP)]
	m.sessMu.RUnlock()
	return has
}

func (m *SessionMgr) Del(id session.ID) {
	m.idsMu.Lock()
	delete(m.ids, id)
	m.idsMu.Unlock()

	m.sessMu.Lock()
	delete(m.ids, id)
	m.sessMu.Unlock()
}
