//go:build linux
// +build linux

package session

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/app/server/proxyer/sender"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
)

type Downlink interface {
	Downlink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

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

func (sm *SessionMgr) Add(proxyerCtx context.Context, down Downlink, s session.Session) (*Session, error) {
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

	locPort, err := sm.ap.GetPort(s.Proto, s.DstAddr)
	if err != nil {
		return nil, err
	}

	ns, err := newSession(
		proxyerCtx, down,
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
	err := errors.Join(
		s.close(cause),
		sm.ap.DelPort(
			s.session.Proto,
			s.locAddr.Port(),
			s.session.DstAddr,
		),
	)

	sm.mu.Lock()
	delete(sm.sessionMap, s.ID())
	delete(sm.idMap, s.session)
	sm.mu.Unlock()

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
		err = errors.Join(
			err,
			sm.del(e, context.Canceled),
		)
	}
	return err
}

type Session struct {
	ctx    cctx.CancelCtx
	closed atomic.Bool

	id      session.ID
	locAddr netip.AddrPort
	session session.Session

	sender sender.Sender

	cnt atomic.Uint32
}

func newSession(
	proxyerCtx context.Context, down Downlink,
	id session.ID, session session.Session,
	locAddr netip.AddrPort,
) (*Session, error) {
	var s = &Session{
		ctx:     cctx.WithContext(proxyerCtx),
		id:      id,
		locAddr: locAddr,
		session: session,
	}

	var err error
	s.sender, err = sender.NewSender(locAddr, session.Proto, session.DstAddr)
	if err != nil {
		return nil, err
	}

	go s.downlinkService(down)
	return s, nil
}

func (s *Session) ID() session.ID {
	return s.id
}

func (s *Session) downlinkService(down Downlink) {
	mtu := down.MTU()
	seg := relraw.NewPacket(0, mtu)

	for {
		seg.Sets(0, mtu)
		err := s.sender.Recv(s.ctx, seg)
		if err != nil {
			s.ctx.Cancel(err)
			return
		}

		switch s.session.Proto {
		case itun.TCP:
			header.TCP(seg.Data()).SetDestinationPortWithChecksumUpdate(s.session.SrcAddr.Port())
		case itun.UDP:
			header.UDP(seg.Data()).SetDestinationPortWithChecksumUpdate(s.session.SrcAddr.Port())
		default:
		}

		err = down.Downlink(seg, s.id)
		if err != nil {
			s.ctx.Cancel(err)
			return
		}
	}
}

func (s *Session) Send(pkt *relraw.Packet) error {
	return s.sender.Send(pkt)
}

func (s *Session) tick() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
	}

	const magic uint32 = 0x45a2319f
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

		err := errors.Join(
			s.ctx.Err(),
			s.sender.Close(),
		)
		fmt.Println(err) // tood: log
		return err
	}
	return nil
}
