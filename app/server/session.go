//go:build linux
// +build linux

package server

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
)

type SessionMgr struct {
	proxyer *proxyer
	idmgr   IdMgr

	sync.RWMutex

	// record mapping session id and Session
	sessions map[uint16]*Session

	// filter reduplicate add session
	ids map[itun.Session]uint16

	idle *itun.Idle
}

func NewSessionMgr(pxyer *proxyer) *SessionMgr {
	mgr := &SessionMgr{
		proxyer: pxyer,

		sessions: make(map[uint16]*Session, 16),
		ids:      make(map[itun.Session]uint16, 16),
	}
	var tick *time.Ticker
	mgr.idle, tick = itun.NewIdle(pxyer.srv.cfg.ProxyerIdeleTimeout)

	// todo: 把keepalive移出去
	go mgr.keepalive(context.Background(), tick)
	return mgr
}

// todo: session 应该包括localAddr， 如果同一个机器的两个程序同时访问了相同的地址时
func (sm *SessionMgr) Add(ctx cctx.CancelCtx, s itun.Session) (*Session, error) {
	sm.RLock()
	id, has := sm.ids[s]
	sm.RUnlock()
	if has {
		return sm.Get(id), nil
	}

	id, err := sm.idmgr.Get()
	if err != nil {
		return nil, err
	}
	port, err := sm.proxyer.srv.ap.GetPort(s.Proto, s.DstAddr)
	if err != nil {
		return nil, err
	}
	session, err := NewSession(
		ctx, id, sm.proxyer.conn, s,
		netip.AddrPortFrom(sm.proxyer.srv.Addr.Addr(), port),
	)
	if err != nil {
		return nil, err
	}

	sm.Lock()
	sm.sessions[id] = session
	sm.ids[s] = id
	sm.Unlock()
	return session, nil
}

func (sm *SessionMgr) Del(id uint16) (err error) {
	s := sm.Get(id)
	if s != nil {
		err = s.Close()
		sm.idmgr.Put(s.ID())

		sm.Lock()
		delete(sm.ids, s.session)
		delete(sm.sessions, id)
		sm.Unlock()
	}
	return err
}

func (sm *SessionMgr) Get(id uint16) *Session {
	sm.RLock()
	defer sm.RUnlock()
	return sm.sessions[id]
}

func (sm *SessionMgr) keepalive(ctx context.Context, ticker *time.Ticker) {
	var ids = make([]uint16, 0, 16)
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}

		sm.RLock()
		for id, s := range sm.sessions {
			if s.Idled() {
				ids = append(ids, id)
				break
			}
		}
		sm.RUnlock()

		for _, id := range ids {
			if err := sm.Del(id); err != nil {
				panic(err)
			}
		}
		ids = ids[:0]
	}
}

const maxSessions = 0xffff - 1

var ErrSessionExceed = errors.New("session exceed limit")

type Session struct {
	ctx     cctx.CancelCtx
	id      uint16
	session itun.Session

	pxy relraw.RawConn

	idle *itun.Idle
}

func NewSession(
	ctx cctx.CancelCtx, id uint16,
	conn *sconn.Conn,
	s itun.Session, laddr netip.AddrPort,
) (*Session, error) {

	var se = &Session{
		ctx: cctx.WithContext(ctx),
		id:  id,
	}

	var err error
	switch s.Proto {
	case itun.TCP:
		se.pxy, err = bpf.Connect(laddr, s.DstAddr, relraw.UsedPort())
		if err != nil {
			return nil, err
		}
	default:
		// todo: udp
		return nil, fmt.Errorf("not support itun number %d", s.Proto)
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
		err := s.pxy.ReadCtx(s.ctx, seg.Packet())
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
		s.idle.Action()
	}
}

// Write write uplink proxy-data
func (s *Session) Write(pxy []byte) error {
	s.idle.Action()
	_, err := s.pxy.Write(pxy)
	return err
}

func (s *Session) Idled() bool {
	return s.idle.Idled()
}

func (s *Session) Close() error {
	err := s.pxy.Close()
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

	return 0, fmt.Errorf("unknown error")
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
