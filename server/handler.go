//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
)

type handler struct {
	ctx        cctx.CancelCtx
	srv        *Server
	raw        *itun.RawConn
	sessionMgr *SessionMgr

	conn *sconn.Conn // 连接 Client <----> ProxyServer 的安全conn

	mgrConn *control.CtrConn
}

// todo: 这个conn是fack tcp, 它不是可靠的, 我们按照数据报来接受处理他
// 它最开始几个数据包的行为符合正常的tcp

func Handle(c context.Context, srv *Server, raw *itun.RawConn) {
	ctx := cctx.WithContext(c)
	var h = &handler{
		ctx: ctx,
		srv: srv,
		raw: raw,
	}
	h.sessionMgr = NewSessionMgr(h)

	// build secret conn and control-conn
	h.conn = sconn.Accept(ctx, raw, &h.srv.cfg.Config)
	if err := ctx.Err(); err != nil {
		fmt.Println(err)
		return
	}
	h.mgrConn = control.AcceptCtrConn(ctx, h.conn)
	if err := ctx.Err(); err != nil {
		fmt.Println(err)
		return
	}

	// start uplink handler
	go h.uplink(ctx)

	// start control handler
	go h.control(ctx)

	<-ctx.Done()
	fmt.Println("proxy closed: ", ctx.Err())

}

func (h *handler) control(ctx cctx.CancelCtx) {
	control.Serve(
		ctx,
		h.mgrConn,
		handlerImplPtr(h),
		time.Second*5, // todo: from config
	)
}

func (h *handler) uplink(ctx cctx.CancelCtx) {
	var b = make([]byte, h.conn.Raw().MTU())
	for {
		seg, err := h.conn.RecvSeg(b)
		if err != nil {
			ctx.Cancel(err)
			return
		}

		if id := seg.ID(); id == segment.CtrSegID {
			h.mgrConn.Inject(seg)
		} else {
			s := h.sessionMgr.Get(id)
			if s != nil {
				err := s.Write(seg.Payload())
				if err != nil {
					ctx.Cancel(err)
					return
				}
			} else {
				fmt.Println("not register session or timeout session, need register")
			}
		}
	}
}
