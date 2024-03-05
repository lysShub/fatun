//go:build linux
// +build linux

package server

import (
	"context"
	"net/netip"
	"sync"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
	pkge "github.com/pkg/errors"
)

type Downlink interface {
	Downlink(b *relraw.Packet, id session.ID)
	MTU() int
}

type SessionMgr struct {
	proxyer *proxyer
	idmgr   *session.IdMgr

	sync.RWMutex

	// record mapping session id and Session
	sessionMap map[session.ID]*Session

	// filter reduplicate add session
	idMap map[session.Session]session.ID

	ka *itun.Keepalive
}

func NewSessionMgr(pxyer *proxyer) *SessionMgr {
	mgr := &SessionMgr{
		proxyer: pxyer,

		sessionMap: make(map[session.ID]*Session, 16),
		idMap:      make(map[session.Session]session.ID, 16),
		ka:         itun.NewKeepalive(pxyer.srv.cfg.ProxyerIdeleTimeout),
	}

	go mgr.keepalive(context.Background())
	return mgr
}

func (sm *SessionMgr) Add(ctx cctx.CancelCtx, session session.Session) (*Session, error) {
	sm.RLock()
	id, has := sm.idMap[session]
	sm.RUnlock()
	if has {
		return sm.Get(id), nil
	}

	id, err := sm.idmgr.Get()
	if err != nil {
		return nil, err
	}
	port, err := sm.proxyer.srv.ap.GetPort(session.Proto, session.DstAddr)
	if err != nil {
		return nil, err
	}
	locAddr := netip.AddrPortFrom(sm.proxyer.srv.Addr.Addr(), port)
	s, err := NewSession(
		ctx, sm.proxyer, id, session,
		locAddr, sm.ka.Task(),
	)
	if err != nil {
		return nil, err
	}

	sm.Lock()
	sm.sessionMap[id] = s
	sm.idMap[session] = id
	sm.Unlock()
	return s, nil
}

func (sm *SessionMgr) Del(id session.ID) (err error) {
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

func (sm *SessionMgr) Get(id session.ID) *Session {
	sm.RLock()
	defer sm.RUnlock()
	return sm.sessionMap[id]
}

func (sm *SessionMgr) keepalive(ctx context.Context) {
	var idleSessions = make([]session.ID, 0, 16)

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

type Session struct {
	ctx     cctx.CancelCtx
	id      session.ID
	session session.Session

	capture relraw.RawConn

	task *itun.Task
}

type sender interface {
	Send(ctx context.Context, b *relraw.Packet, id session.ID)
	MTU() int
}

func NewSession(
	ctx cctx.CancelCtx, down Downlink,
	id session.ID, session session.Session,
	locAddr netip.AddrPort, task *itun.Task,
) (*Session, error) {

	var se = &Session{
		ctx:     cctx.WithContext(ctx),
		id:      id,
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

	go se.recvService(down)
	return se, nil
}

func (s *Session) ID() session.ID {
	return s.id
}

// recv from server and write to raw
func (s *Session) recvService(sdr Downlink) {
	mtu := sdr.MTU()
	b := relraw.NewPacket(
		64, mtu, 16,
	)

	for {
		b.Sets(0, mtu)
		err := s.capture.ReadCtx(s.ctx, b)
		if err != nil {
			s.ctx.Cancel(err)
			return
		}

		switch s.session.Proto {
		case itun.TCP:
			header.TCP(b.Data()).SetDestinationPortWithChecksumUpdate(s.session.SrcAddr.Port())
		case itun.UDP:
			header.UDP(b.Data()).SetDestinationPortWithChecksumUpdate(s.session.SrcAddr.Port())
		default:
		}

		sdr.Downlink(b, s.id)
		// if err != nil {
		// 	s.ctx.Cancel(err)
		// 	return
		// }
		s.task.Action()
	}
}

// Send write uplink proxy-data
func (s *Session) Send(b []byte) error {
	s.task.Action()
	_, err := s.capture.Write(b)
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
