//go:build linux
// +build linux

package server

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"sync"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
	pkge "github.com/pkg/errors"
)

type SessionMgr struct {
	proxyer *proxyer
	idmgr   IdMgr

	sync.RWMutex

	// record mapping session id and Session
	sessionMap map[uint16]*Session

	// filter reduplicate add session
	idMap map[itun.Session]uint16

	ka *itun.Keepalive
}

func NewSessionMgr(pxyer *proxyer) *SessionMgr {
	mgr := &SessionMgr{
		proxyer: pxyer,

		sessionMap: make(map[uint16]*Session, 16),
		idMap:      make(map[itun.Session]uint16, 16),
		ka:         itun.NewKeepalive(pxyer.srv.cfg.ProxyerIdeleTimeout),
	}

	go mgr.keepalive(context.Background())
	return mgr
}

func (sm *SessionMgr) Add(ctx cctx.CancelCtx, session itun.Session) (*Session, error) {
	sm.RLock()
	sessionId, has := sm.idMap[session]
	sm.RUnlock()
	if has {
		return sm.Get(sessionId), nil
	}

	sessionId, err := sm.idmgr.Get()
	if err != nil {
		return nil, err
	}
	port, err := sm.proxyer.srv.ap.GetPort(session.Proto, session.DstAddr)
	if err != nil {
		return nil, err
	}
	locAddr := netip.AddrPortFrom(sm.proxyer.srv.Addr.Addr(), port)
	s, err := NewSession(
		ctx, sm.proxyer.conn, sessionId, session,
		locAddr, sm.ka.Task(),
	)
	if err != nil {
		return nil, err
	}

	sm.Lock()
	sm.sessionMap[sessionId] = s
	sm.idMap[session] = sessionId
	sm.Unlock()
	return s, nil
}

func (sm *SessionMgr) Del(id uint16) (err error) {
	s := sm.Get(id)
	if s != nil {
		err = s.Close()
		sm.idmgr.Put(s.ID())

		sm.Lock()
		delete(sm.idMap, s.session)
		delete(sm.sessionMap, id)
		sm.Unlock()
	}
	return err
}

func (sm *SessionMgr) Get(id uint16) *Session {
	sm.RLock()
	defer sm.RUnlock()
	return sm.sessionMap[id]
}

func (sm *SessionMgr) keepalive(ctx context.Context) {
	var idleSessions = make([]uint16, 0, 16)

	ticker := sm.ka.Ticker()
	for {
		select {
		case <-ticker:
		case <-ctx.Done():
			return
		}

		sm.RLock()
		for id, s := range sm.sessionMap {
			if s.Idle() {
				idleSessions = append(idleSessions, id)
				if len(idleSessions) == cap(idleSessions) {
					break
				}
			}
		}
		sm.RUnlock()

		for _, id := range idleSessions {
			if err := sm.Del(id); err != nil {
				panic(err)
			}
		}
		idleSessions = idleSessions[:0]
	}
}

const maxSessions = 0xffff - 1

var ErrSessionExceed = errors.New("session exceed limit")

type Session struct {
	ctx     cctx.CancelCtx
	id      uint16
	session itun.Session

	capture relraw.RawConn

	task *itun.Task
}

func NewSession(
	ctx cctx.CancelCtx, conn *sconn.Conn,
	sessionId uint16, session itun.Session,
	locAddr netip.AddrPort, task *itun.Task,
) (*Session, error) {

	var se = &Session{
		ctx:     cctx.WithContext(ctx),
		id:      sessionId,
		session: session,
		task:    task,
	}

	var err error
	switch session.Proto {
	case itun.TCP:
		se.capture, err = bpf.Connect(locAddr, session.DstAddr, relraw.UsedPort())
		if err != nil {
			return nil, err
		}
	default:
		// todo: udp
		return nil, pkge.Errorf("not support itun number %d", session.Proto)
	}

	go se.downlink(conn)
	return se, nil
}

func (s *Session) ID() uint16 {
	return s.id
}

// recv from server and write to raw
func (s *Session) downlink(conn *sconn.Conn) {
	mtu := conn.Raw().MTU()
	seg := segment.ToSegment(relraw.NewPacket(
		64, mtu, 16,
	))

	for {
		seg.Sets(0, mtu)
		err := s.capture.ReadCtx(s.ctx, seg.Packet())
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

		seg.SetID(s.id)

		err = conn.SendSeg(s.ctx, seg)
		if err != nil {
			s.ctx.Cancel(err)
			return
		}
		s.task.Action()
	}
}

// Write write uplink proxy-data
func (s *Session) Write(pxy []byte) error {
	s.task.Action()
	_, err := s.capture.Write(pxy)
	return err
}

func (s *Session) Idle() bool {
	return s.task.Idle()
}

func (s *Session) Close() error {
	err := s.capture.Close()
	s.ctx.Cancel(nil)
	return err
}

type IdMgr struct {
	mu     sync.RWMutex
	allocs []uint16 // asc
}

func (m *IdMgr) Get() (uint16, error) {
	m.mu.RLock()
	id, err := m.getLocked()
	m.mu.RUnlock()
	if err != nil {
		return 0, err
	}

	m.mu.Lock()
	m.allocs = append(m.allocs, id)
	slices.Sort(m.allocs)
	m.mu.Unlock()

	return id, nil
}

func (m *IdMgr) getLocked() (id uint16, err error) {
	n := len(m.allocs)
	if n >= maxSessions {
		return 0, ErrSessionExceed
	} else if n == 0 {
		return 0, nil
	}

	id = m.allocs[n-1] + 1
	if id != segment.CtrSegID && !slices.Contains(m.allocs, id) {
		return id, nil
	}
	for i := 0; i < n-1; i++ {
		if m.allocs[i]+1 != m.allocs[i+1] {
			return m.allocs[i] + 1, nil
		}
	}

	return 0, pkge.Errorf("unknown error")
}

func (m *IdMgr) Put(id uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()

	i := slices.Index(m.allocs, id)
	if i < 0 {
		return
	}

	m.allocs[i] = segment.CtrSegID
	slices.Sort(m.allocs)
	m.allocs = m.allocs[:len(m.allocs)-1]
}
