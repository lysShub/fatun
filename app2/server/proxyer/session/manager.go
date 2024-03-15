package session

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
)

type SessionMgr struct {
	idmgr *session.IdMgr
	ap    *adapter.Ports
	addr  netip.Addr

	ticker  *time.Ticker
	closeCh chan struct{}

	mu sync.RWMutex
	// record mapping session id and Session
	sessionMap map[session.ID]*Session
	// filter reduplicate add session
	idMap map[session.Session]session.ID
}

func NewSessionMgr(ap *adapter.Ports, locAddr netip.Addr) *SessionMgr {
	mgr := &SessionMgr{
		idmgr: session.NewIDMgr(),
		ap:    ap,
		addr:  locAddr,

		ticker:  time.NewTicker(time.Minute), // todo: from config
		closeCh: make(chan struct{}),

		sessionMap: make(map[session.ID]*Session, 16),
		idMap:      make(map[session.Session]session.ID, 16),
	}

	go mgr.keepalive()
	return mgr
}

func (sm *SessionMgr) Get(id session.ID) (s *Session, err error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s = sm.sessionMap[id]

	if s == nil {
		err = session.ErrInvalidID(id)
	}
	return s, err
}

type PortAlloc interface {
	GetPort(proto itun.Proto, dst netip.AddrPort) (port uint16, err error)
}

func (sm *SessionMgr) Add(proxyer Proxyer, s session.Session) (*Session, error) {
	sm.mu.RLock()
	_, has := sm.idMap[s]
	sm.mu.RUnlock()
	if has {
		return nil, session.ErrExistSession(s)
	}

	id, err := sm.idmgr.Get()
	if err != nil {
		return nil, err
	}

	locPort, err := sm.ap.GetPort(s.Proto, s.Dst)
	if err != nil {
		return nil, err
	}

	ns, err := newSession(
		proxyer,
		id, s,
		netip.AddrPortFrom(sm.addr, locPort),
	)
	if err != nil {
		return nil, err
	}

	sm.mu.Lock()
	sm.sessionMap[id] = ns
	sm.idMap[s] = id
	sm.mu.Unlock()
	return ns, nil
}

func (sm *SessionMgr) Del(id session.ID, cause error) (err error) {
	s, err := sm.Get(id)
	if err != nil {
		return err
	} else {
		return sm.del(s, cause)
	}
}

func (sm *SessionMgr) del(s *Session, cause error) error {
	sm.mu.Lock()
	delete(sm.sessionMap, s.ID())
	delete(sm.idMap, s.session)
	sm.mu.Unlock()

	err := errorx.Join(
		s.close(cause),
		sm.ap.DelPort(
			s.session.Proto,
			s.locAddr.Port(),
			s.session.Dst,
		),
	)
	return err
}

func (sm *SessionMgr) keepalive() {
	var ss = make([]*Session, 0, 8)

	for {
		ss = ss[:0]
		select {
		case <-sm.ticker.C:
			sm.mu.RLock()
			for _, e := range sm.sessionMap {
				if e.tick() {
					ss = append(ss, e)
				}
			}
			sm.mu.RUnlock()
		case <-sm.closeCh:
			return
		}

		for _, e := range ss {
			sm.del(e, itun.KeepaliveExceeded)
		}
	}
}

func (sm *SessionMgr) Close() (err error) {
	select {
	case <-sm.closeCh:
		return nil // closed
	default:
		close(sm.closeCh)
		sm.ticker.Stop()
	}

	var ss = make([]*Session, 0, len(sm.sessionMap))
	sm.mu.Lock()
	for _, e := range sm.sessionMap {
		ss = append(ss, e)
	}
	clear(sm.sessionMap)
	clear(sm.idMap)
	sm.mu.Unlock()

	for _, e := range ss {
		err = errorx.Join(
			err,
			sm.del(e, context.Canceled),
		)
	}
	return err
}
