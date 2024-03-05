package client

import (
	"sync"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
	pkge "github.com/pkg/errors"
)

type Uplink interface {
	Uplink(b *relraw.Packet, id session.ID)
	MTU() int
}

type SessionMgr struct {
	client *Client

	keepalive *itun.Keepalive
	ticker    *time.Ticker

	mu       sync.RWMutex
	sessions map[session.ID]*Session
}

func NewSessionMgr(c *Client) *SessionMgr {
	return &SessionMgr{
		client: c,
		// keepalive: itun.NewKeepalive(),
	}
}

func (sm *SessionMgr) Add(s session.Session, id session.ID) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, has := sm.sessions[id]; has {
		return pkge.Errorf("id %d exist", id)
	}

	session, err := NewSession(sm.client.ctx, sm.client, id, s)
	if err != nil {
		return err
	}

	sm.sessions[id] = session

	return nil
}

func (sm *SessionMgr) Get(id session.ID) *Session {
	sm.mu.RLock()
	defer sm.mu.RLock()
	return sm.sessions[id]
}

type Session struct {
	ctx     cctx.CancelCtx
	session session.Session
	id      session.ID

	capture Capture
}

func NewSession(
	ctx cctx.CancelCtx, up Uplink,
	id session.ID, session session.Session,
) (*Session, error) {
	var s = &Session{
		ctx:     ctx,
		id:      id,
		session: session,
	}

	var err error
	s.capture, err = NewCapture(session)
	if err != nil {
		return nil, err
	}

	go s.uplinkService(up)
	return s, nil
}

func (s *Session) uplinkService(up Uplink) {
	var mtu = up.MTU()
	p := relraw.NewPacket(0, mtu)

	for {
		p.Sets(0, mtu)
		if err := s.capture.RecvCtx(s.ctx, p); err != nil {
			s.ctx.Cancel(err)
			return
		}

		// todo: reset tcp mss

		up.Uplink(p, session.ID(s.id))

	}
}

func (s *Session) Inject(b *relraw.Packet) error {
	// if seg.ID() != s.id {
	// 	return pkge.Errorf("expect session %d, got %d", s.id, seg.ID())
	// }

	b.SetHead(b.Head() + session.Size)

	return s.capture.Inject(b)
}
