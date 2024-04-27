package session

import (
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/lysShub/fatun/app/server/adapter"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
)

type SessionMgr struct {
	idmgr   *session.IdMgr
	proxyer Proxyer

	ticker  *time.Ticker
	closeCh chan struct{}

	mu sync.RWMutex
	// record mapping session id and Session
	sessionMap map[session.ID]*Session
	// filter reduplicate add session
	idMap map[session.Session]session.ID
}

type Proxyer interface {
	Logger() *slog.Logger

	Addr() netip.AddrPort

	Keepalive() time.Duration

	Adapter() *adapter.Ports

	Downlink(pkt *packet.Packet, id session.ID) error

	MTU() int
}

func NewSessionMgr(proxyer Proxyer) *SessionMgr {
	mgr := &SessionMgr{
		idmgr:   session.NewIDMgr(),
		proxyer: proxyer,
		closeCh: make(chan struct{}),

		sessionMap: make(map[session.ID]*Session, 16),
		idMap:      make(map[session.Session]session.ID, 16),
	}

	return mgr
}

func (mgr *SessionMgr) Get(id session.ID) (s *Session, err error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	s = mgr.sessionMap[id]

	if s == nil {
		err = session.ErrInvalidID(id)
	}
	return s, err
}

func (mgr *SessionMgr) Add(s session.Session) (*Session, error) {
	mgr.mu.RLock()
	_, has := mgr.idMap[s]
	mgr.mu.RUnlock()
	if has {
		return nil, session.ErrSessionExist(s)
	}

	id, err := mgr.idmgr.Get()
	if err != nil {
		return nil, err
	}

	ns, err := newSession(
		mgr, mgr.proxyer,
		id, s,
	)
	if err != nil {
		return nil, err
	}

	mgr.mu.Lock()
	mgr.sessionMap[id] = ns
	mgr.idMap[s] = id
	mgr.mu.Unlock()
	return ns, nil
}

func (mgr *SessionMgr) Del(id session.ID, cause error) (err error) {
	s, err := mgr.Get(id)
	if err != nil {
		return err
	} else {
		mgr.del(s)
		return nil
	}
}

func (mgr *SessionMgr) del(s *Session) {
	mgr.mu.Lock()
	delete(mgr.sessionMap, s.ID())
	delete(mgr.idMap, s.session)
	mgr.mu.Unlock()
}

func (mgr *SessionMgr) Close() (err error) {
	select {
	case <-mgr.closeCh:
		return nil // closed
	default:
		close(mgr.closeCh)
		mgr.ticker.Stop()
	}

	var ss = make([]*Session, 0, len(mgr.sessionMap))
	mgr.mu.Lock()
	for _, e := range mgr.sessionMap {
		ss = append(ss, e)
	}
	clear(mgr.sessionMap)
	clear(mgr.idMap)
	mgr.mu.Unlock()

	for _, e := range ss {
		mgr.del(e)
		e.close(os.ErrClosed)
	}
	return err
}
