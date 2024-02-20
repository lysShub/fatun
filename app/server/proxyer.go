//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
)

type proxyer struct {
	ctx        cctx.CancelCtx
	srv        *Server
	sessionMgr *SessionMgr

	SrcAddr netip.AddrPort
	conn    *sconn.Conn // 连接 Client <----> ProxyServer 的安全conn

	ctrConn *control.CtrConn
	ctr     *control.Server
}

// todo: 这个conn是fack tcp, 它不是可靠的, 我们按照数据报来接受处理他
// 它最开始几个数据包的行为符合正常的tcp

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	ctx := cctx.WithContext(c)
	var h = &proxyer{
		ctx:     ctx,
		srv:     srv,
		SrcAddr: raw.RemoteAddrPort(),
	}
	h.sessionMgr = NewSessionMgr(h)

	// build secret conn and control-conn
	h.conn = sconn.Accept(ctx, raw, &h.srv.cfg.Sconn)
	if err := ctx.Err(); err != nil {
		fmt.Println(err)
		return
	}

	// control
	h.ctrConn = control.AcceptCtrConn(ctx, h.conn)
	if err := ctx.Err(); err != nil {
		fmt.Println(err)
		return
	}
	h.ctr = control.NewServer(ctx, h.ctrConn, proxyerImplPtr(h))
	if err := ctx.Err(); err != nil {
		fmt.Println(err)
		return
	}

	// start uplink handler
	go h.uplink()

	// start control handler
	go h.control()

	<-ctx.Done()
	fmt.Println("proxy closed: ", ctx.Err())

}

func (p *proxyer) control() {
	p.ctr.Serve(time.Second * 5) // todo: from config
}

func (p *proxyer) uplink() {
	n := p.conn.Raw().MTU()
	seg := segment.NewSegment(n)

	for {
		seg.Packet().Sets(0, n)

		err := p.conn.RecvSeg(p.ctx, seg)
		if err != nil {
			p.ctx.Cancel(err)
			return
		}

		if id := seg.ID(); id == segment.CtrSegID {
			p.ctrConn.Inject(seg)
		} else {
			s := p.sessionMgr.Get(id)
			if s != nil {
				err := s.Write(seg.Payload())
				if err != nil {
					p.ctx.Cancel(err)
					return
				}
			} else {
				fmt.Println("not register session or timeout session, need register")
			}
		}
	}
}
