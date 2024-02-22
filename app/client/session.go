package client

import (
	"fmt"
	"sync"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"github.com/lysShub/relraw"
)

type SessionMgr struct {
	client *Client

	mu sync.RWMutex

	sess map[uint16]*Session
}

func NewSessionMgr(c *Client) *SessionMgr {
	return &SessionMgr{}
}

func (sm *SessionMgr) Add(s itun.Session, id uint16) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, has := sm.sess[id]; has {
		return fmt.Errorf("id %d exist", id)
	}

	session, err := NewSession(sm.client.ctx, sm.client.conn, id, s)
	if err != nil {
		return err
	}

	sm.sess[id] = session

	return nil
}

func (sm *SessionMgr) Get(id uint16) *Session {
	sm.mu.RLock()
	defer sm.mu.RLock()
	return sm.sess[id]
}

type Session struct {
	ctx cctx.CancelCtx
	s   itun.Session
	id  uint16

	// todo: add idle

	cpt Capture
}

func NewSession(
	ctx cctx.CancelCtx, conn *sconn.Conn,
	id uint16, session itun.Session,
) (*Session, error) {
	var s = &Session{
		ctx: ctx,
		id:  id,
		s:   session,
	}

	var err error
	s.cpt, err = NewCapture(session)
	if err != nil {
		return nil, err
	}

	go s.uplink(conn)
	return s, nil
}

func (s *Session) uplink(conn *sconn.Conn) {
	var mtu = conn.Raw().MTU()
	p := relraw.NewPacket(64, mtu)

	for {
		p.Sets(64, mtu)
		if err := s.cpt.RecvCtx(s.ctx, p); err != nil {
			s.ctx.Cancel(err)
			return
		}

		// todo: reset tcp mss

		seg := segment.ToSegment(p)
		seg.SetID(s.id)
		if err := conn.SendSeg(s.ctx, seg); err != nil {
			s.ctx.Cancel(err)
			return
		}
	}
}

func (s *Session) Inject(seg *segment.Segment) error {
	if seg.ID() != s.id {
		return fmt.Errorf("expect session %d, got %d", s.id, seg.ID())
	}

	// decode segment
	seg.SetHead(seg.Head() + segment.HdrSize)

	return s.cpt.Inject(seg.Packet())
}
