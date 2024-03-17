package session

import (
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type SessionMgr struct {
	idmgr   *session.IdMgr
	ap      *adapter.Ports
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
	// logger
	Logger() *slog.Logger

	Addr() netip.AddrPort

	Keepalive() time.Duration

	// proxyer downlink
	Downlink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

func NewSessionMgr(ap *adapter.Ports, pxy Proxyer) *SessionMgr {
	mgr := &SessionMgr{
		idmgr: session.NewIDMgr(),
		ap:    ap,

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
		return nil, session.ErrExistSession(s)
	}

	id, err := mgr.idmgr.Get()
	if err != nil {
		return nil, err
	}

	ns, err := newSession(
		mgr,
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
		return mgr.del(s)
	}
}

func (mgr *SessionMgr) del(s *Session) error {
	mgr.mu.Lock()
	delete(mgr.sessionMap, s.ID())
	delete(mgr.idMap, s.session)
	mgr.mu.Unlock()

	err := mgr.ap.DelPort(
		s.session.Proto,
		s.locAddr.Port(),
		s.session.Dst,
	)
	return err
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
		err = errorx.Join(
			err,
			mgr.del(e),
		)
		e.close(os.ErrClosed)
	}
	return err
}
