package client

import (
	"sync"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/protocol"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
)

type SessionMgr struct {
	client *Client

	mu sync.RWMutex

	sess map[uint16]*Session
}

func NewSessionMgr(c *Client) *SessionMgr {
	return &SessionMgr{}
}

func (sm *SessionMgr) Add(s protocol.Session, id uint16) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := NewSession(sm.client.conn)

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
	s   protocol.Session

	capture Capture
}

func NewSession(

	conn *sconn.Conn,

) *Session {
	var s = &Session{}

	go s.uplink(conn)
	return s
}

func (s *Session) uplink(conn *sconn.Conn) {
	// var b = make([]byte, conn.Raw().MTU())

	// for {
	// n, err := s.capture.RecvCtx(s.ctx, b)
	// if err != nil {
	// 	s.ctx.Cancel(err)
	// 	return
	// }

	// conn.SendSeg(b[:n], 0)
	// }

}

func (s *Session) Inject(seg *segment.Segment) error {
	return s.capture.Inject(nil) // todo:
}
