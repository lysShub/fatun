//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"net/netip"

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

	ctrRaw control.CtrInject
}

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	var p = &proxyer{
		ctx:     cctx.WithContext(c),
		srv:     srv,
		SrcAddr: raw.RemoteAddrPort(),
	}
	p.sessionMgr = NewSessionMgr(p)

	p.conn = sconn.Accept(p.ctx, srv.cfg.TCPHandshakeTimeout, raw, &p.srv.cfg.Sconn)
	if err := p.ctx.Err(); err != nil {
		panic(err)
	}

	p.ctrRaw = control.Serve(p.ctx, srv.cfg.TCPHandshakeTimeout, srv.cfg.InitCfgTimeout, p.conn, proxyerImplPtr(p))
	if err := p.ctx.Err(); err != nil {
		panic(err)
	}

	// start uplink handler
	go p.uplink()

	<-p.ctx.Done()
	e := p.ctx.Err()
	fmt.Println("proxy closed: ", e)
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
			// remove ip header and segment header
			pkg := seg.Packet()
			pkg.SetHead(pkg.Head() + segment.HdrSize)

			p.ctrRaw.Inject(pkg)
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
