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
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type proxyer struct {
	ctx        cctx.CancelCtx
	srv        *Server
	sessionMgr *SessionMgr

	SrcAddr netip.AddrPort

	conn *sconn.Conn
}

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	var p = &proxyer{
		ctx:     cctx.WithContext(c),
		srv:     srv,
		SrcAddr: raw.RemoteAddrPort(),
	}
	p.sessionMgr = NewSessionMgr(p)

	p.conn = sconn.Accept(p.ctx, raw, &p.srv.cfg.Sconn)
	if err := p.ctx.Err(); err != nil {
		panic(err)
	}

	ctr, err := control.NewController(raw.LocalAddrPort(), raw.RemoteAddrPort(), raw.MTU())
	if err != nil {
		panic(err)
	}

	go ctr.OutboundService(p.ctx, p.conn)
	go p.uplink(ctr)
	go control.Serve(p.ctx, ctr, proxyerImplPtr(p))

	<-p.ctx.Done()
	e := p.ctx.Err()
	fmt.Println("proxy closed: ", e)
}

func (p *proxyer) uplink(ctrSessionInbound *control.Controller) {
	n := p.conn.Raw().MTU()
	b := relraw.NewPacket(0, n)

	for {
		b.Sets(0, n)

		id, err := p.conn.RecvSeg(p.ctx, b)
		if err != nil {
			p.ctx.Cancel(err)
			return
		}

		if id == session.CtrSessID {
			ctrSessionInbound.Inbound(b)
		} else {
			s := p.sessionMgr.Get(uint16(id))
			if s != nil {
				err := s.Write(b.Data())
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
