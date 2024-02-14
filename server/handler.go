//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
)

type handler struct {
	srv *Server

	conn *sconn.Conn // 连接 Client <----> ProxyServer 的安全conn

	mgrConn *sconn.MgrConn

	//
	sessionMgr *SessionMgr
}

// todo: 这个conn是fack tcp, 它不是可靠的, 我们按照数据报来接受处理他
// 它最开始几个数据包的行为符合正常的tcp

func Handle(ctx context.Context, srv *Server, conn *itun.RawConn) error {
	ctx = context.WithoutCancel(ctx)
	// todo: with timeout ctx

	var h = &handler{srv: srv}
	if err := h.handshake(ctx, conn); err != nil {
		return err
	}

	// manager work
	for {
		seg, err := h.mgrConn.Next()
		if err != nil {
			panic(err)
		}

		t := seg.Type()
		if !t.Validate() || t.IsConfig() {
			panic("invalid manager type ")
		}
		switch t {
		case segment.MgrSegAddTCP:
			// addr, err := seg.AddTCP()
			// if err != nil {
			// 	panic(err)
			// }
			// s, err := h.sessionMgr.Add(ctx, h.conn, itun.Session{Proto: header.TCPProtocolNumber, Server: addr})
			// if err != nil {
			// 	panic(err)
			// }

			// err = h.mgrConn.Replay(segment.MgrAddTCP(s.ID()))
			// if err != nil {
			// 	panic(err)
			// }
		case segment.MgrSegDelTCP:
		case segment.MgrSegAddUDP:
		case segment.MgrSegDelUDP:
		case segment.MgrSegPackLoss:
		case segment.MgrSegPing:
		default:
			panic("todo:")
		}
	}
}

func (h *handler) handshake(ctx context.Context, raw *itun.RawConn) (err error) {
	// prev-send packets and swap secret key stage
	h.conn, err = sconn.Accept(ctx, raw, &h.srv.cfg.Config)
	if err != nil {
		return err
	}

	// establish a manager connection
	if h.mgrConn, err = sconn.AcceptMgrConn(ctx, h.conn); err != nil {
		return err
	}
	go h.uplink(ctx)

	// init manager config stage
	h.initConfig(ctx)

	return nil
}

func (h *handler) initConfig(ctx context.Context) (err error) {
	// todo: set max loop
	for {
		seg, err := h.mgrConn.Next()
		if err != nil {
			return err
		}

		t := seg.Type()
		if !t.IsConfig() {
			return fmt.Errorf("invalid init manager config segment type %s", t)
		}
		switch t {
		case segment.MgrSegIPv6:
			// h.mgrConn.Replay(segment.MgrIPv6(false))
		case segment.MgrSegCrypto:
			// h.mgrConn.Replay(segment.MgrCrypto(h.srv.cfg.Crypto))
		case segment.MgrSegEndConfig:
			return nil
		default:
			panic(fmt.Errorf("handle %s manager segment", t))
		}
	}
}

func (h *handler) uplink(ctx context.Context) {
	var b = make([]byte, h.conn.MTU())

	for {
		seg, err := h.conn.Read(b)
		if err != nil {
			panic(err)
		}

		if id := seg.ID(); id == segment.MgrSegID {
			h.mgrConn.Inject(seg)
		} else {
			s := h.sessionMgr.Get(id)
			if s != nil {
				if err := s.Write(seg.Payload()); err != nil {
					panic(err)
				}
			} else {
				// todo: log
			}
		}
	}
}
