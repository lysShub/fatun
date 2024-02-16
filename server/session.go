//go:build linux
// +build linux

package server

import (
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type SessionMgr struct {
	hdr   *handler
	idmgr idmgr

	sync.RWMutex

	// record mapping session id and Session
	sessions map[uint16]*Session

	// filter reduplicate add session
	ids map[itun.Session]uint16

	idleTrigger *time.Ticker
}

func NewSessionMgr(hdr *handler) *SessionMgr {
	mgr := &SessionMgr{
		hdr: hdr,

		sessions: make(map[uint16]*Session, 16),
		ids:      make(map[itun.Session]uint16, 16),
	}

	go mgr.keepalive()
	return mgr
}

func (sm *SessionMgr) Add(ctx cctx.CancelCtx, session itun.Session) (*Session, error) {
	sm.RLock()
	id, has := sm.ids[session]
	sm.RUnlock()
	if has {
		return sm.Get(id), nil
	}

	id, err := sm.idmgr.Get()
	if err != nil {
		return nil, err
	}
	port, err := sm.hdr.srv.ap.GetPort(session)
	if err != nil {
		return nil, err
	}
	s, err := NewSession(
		ctx, id, sm.hdr.conn, session,
		netip.AddrPortFrom(sm.hdr.srv.Addr.Addr(), port),
	)
	if err != nil {
		return nil, err
	}

	sm.Lock()
	sm.sessions[id] = s
	sm.ids[session] = id
	sm.Unlock()
	return s, nil
}

func (sm *SessionMgr) Del(id uint16) error {
	return nil
}

func (sm *SessionMgr) Get(id uint16) *Session {
	sm.RLock()
	defer sm.RUnlock()
	return sm.sessions[id]
}

func (sm *SessionMgr) keepalive() {
	const keepalive = time.Minute
	const N = 8

	var d = keepalive / 8
	d = d - d.Truncate(time.Second) + time.Second
	sm.idleTrigger = time.NewTicker(d)

	for {
		<-sm.idleTrigger.C

		var sid uint16
		sm.RLock()
		for id, s := range sm.sessions {
			if s.idleTrigger() > N {
				sid = id
				break
			}
		}
		sm.RUnlock()

		if err := sm.Del(sid); err != nil {
			panic(err)
		}
	}
}

const maxSessions = 0xffff - 1

type ErrSessionExceed struct{}

func (e ErrSessionExceed) Error() string {
	return "session exceed limit"
}

type Session struct {
	ctx cctx.CancelCtx
	pxy relraw.RawConn
	id  uint16

	idleCnt       uint8
	lastKeepalive uint16
	keepalive     uint16
}

func NewSession(
	ctx cctx.CancelCtx, id uint16,
	raw *sconn.Conn,
	s itun.Session, laddr netip.AddrPort,
) (*Session, error) {

	var se = &Session{
		ctx: cctx.WithContext(ctx),
		id:  id,
	}

	var err error
	switch s.Proto {
	case header.TCPProtocolNumber:
		se.pxy, err = bpf.Connect(laddr, s.Server, relraw.UsedPort())
		if err != nil {
			return nil, err
		}
	default:
		// todo: udp
		return nil, fmt.Errorf("not support protocol number %d", s.Proto)
	}

	go se.downlink(raw)
	return se, nil
}

func (s *Session) ID() uint16 {
	return s.id
}

// recv from s and write to raw
func (s *Session) downlink(conn *sconn.Conn) {
	var b = make([]byte, conn.Raw().MTU())

	for {
		n, err := s.pxy.ReadCtx(s.ctx, b)
		if err != nil {
			s.ctx.Cancel(err)
			return
		}

		iphdr := header.IPv4(b[:n]) // todo: ipv4
		i := int(iphdr.HeaderLength()) - 1
		segment.Segment(iphdr[i:]).SetID(s.id)

		err = conn.SendSeg(segment.Segment(iphdr), i)
		if err != nil {
			s.ctx.Cancel(err)
			return
		}
	}
}

// Write write uplink proxy-data
func (s *Session) Write(pxy []byte) error {
	_, err := s.pxy.Write(pxy)
	return err
}

func (s *Session) idleTrigger() uint8 {
	if s.lastKeepalive == s.keepalive {
		s.idleCnt++
	} else {
		s.idleCnt = 0
	}
	s.lastKeepalive, s.keepalive = s.keepalive, 0

	return s.idleCnt
}

func (s *Session) Close() error {
	s.ctx.Cancel(nil)
	s.pxy.Close()
	return nil // todo:
}

type idmgr struct {
	sync.RWMutex
	allocs []uint16 // asc
}

func (m *idmgr) Get() (uint16, error) {
	m.RLock()
	id, err := m.getLocked()
	m.RUnlock()
	if err != nil {
		return 0, err
	}

	m.Lock()
	m.allocs = append(m.allocs, id)
	slices.Sort(m.allocs)
	m.Unlock()

	return id, nil
}

func (m *idmgr) getLocked() (id uint16, err error) {
	n := len(m.allocs)
	if n >= maxSessions {
		return 0, ErrSessionExceed{}
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

	return 0, fmt.Errorf("unknown error")
}

func (m *idmgr) Put(id uint16) {
	m.Lock()
	defer m.Unlock()

	i := slices.Index(m.allocs, id)
	if i < 0 {
		return
	}

	m.allocs[i] = segment.CtrSegID
	slices.Sort(m.allocs)
	m.allocs = m.allocs[:len(m.allocs)-1]
}
