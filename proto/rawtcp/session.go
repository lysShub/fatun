package rawtcp

import (
	"itun/proto"
	"sync"
)

type SessionMgr struct {
	sessionMapMu sync.RWMutex
	sessionMap   map[proto.Session]uint16

	idMapMu sync.RWMutex
	idMap   map[uint16]proto.Session
}

func (p *SessionMgr) GetID(s proto.Session) (id uint16) {
	p.sessionMapMu.RLock()
	id = p.sessionMap[s]
	p.sessionMapMu.RUnlock()
	return id
}

func (p *SessionMgr) GetSession(id uint16) (s proto.Session) {
	p.idMapMu.Lock()
	s = p.idMap[id]
	p.idMapMu.RUnlock()
	return s
}

func (p *SessionMgr) DelID(id uint16) {
	var s proto.Session
	p.idMapMu.Lock()
	s = p.idMap[id]
	delete(p.idMap, id)
	p.idMapMu.Unlock()

	p.sessionMapMu.Lock()
	delete(p.sessionMap, s)
	p.sessionMapMu.Unlock()
}

func (p *SessionMgr) DelSession(s proto.Session) {
	var id uint16
	p.sessionMapMu.Lock()
	id = p.sessionMap[s]
	delete(p.sessionMap, s)
	p.sessionMapMu.Unlock()

	p.idMapMu.Lock()
	delete(p.idMap, id)
	p.idMapMu.Unlock()
}

func (p *SessionMgr) Add(s proto.Session, id uint16) {
	p.sessionMapMu.Lock()
	p.sessionMap[s] = id
	p.sessionMapMu.Unlock()

	p.idMapMu.Lock()
	p.idMap[id] = s
	p.idMapMu.Unlock()
}
