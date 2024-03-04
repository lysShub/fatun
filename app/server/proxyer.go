//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"github.com/stretchr/testify/require"
)

type proxyer struct {
	ctx        cctx.CancelCtx
	srv        *Server
	sessionMgr *SessionMgr

	raw *itun.RawConn

	SrcAddr netip.AddrPort

	inited bool

	seq, ack uint32

	fake   *faketcp.FakeTCP
	crypto *crypto.TCP
}

var _ app.Sender = (*proxyer)(nil)

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	var p = &proxyer{
		ctx:     cctx.WithContext(c),
		srv:     srv,
		SrcAddr: raw.RemoteAddrPort(),

		raw: raw,
	}
	p.sessionMgr = NewSessionMgr(p)

	// p.conn = sconn.Accept(p.ctx, raw, &p.srv.cfg.Sconn)
	// if err := p.ctx.Err(); err != nil {
	// 	panic(err)
	// }

	// ctr, err := control.NewController(raw.LocalAddrPort(), raw.RemoteAddrPort(), raw.MTU())
	// if err != nil {
	// 	panic(err)
	// }

	// go ctr.OutboundService(p.ctx, p.conn)
	// go p.uplink(ctr)
	// go control.Serve(p.ctx, ctr, proxyerImplPtr(p))

	<-p.ctx.Done()
	e := p.ctx.Err()
	fmt.Println("proxy closed: ", e)
}

func (p *proxyer) recvService() {
	n := p.raw.MTU()

	b := relraw.NewPacket(0, n)
	for {
		b.Sets(0, n)

		err := p.raw.ReadCtx(p.ctx, b)
		if err != nil {
			p.ctx.Cancel(err)
			return
		} else if b.Len() == 0 {
			continue
		}

		if faketcp.IsFakeTCP(b.Data()) {
			err = p.crypto.Decrypt(b)
			if err != nil {
				panic(err)
			}

			id := session.GetID(b)
			if id == session.CtrSessID {
				// todo: attach ip hdr
				p.srv.st.Inbound(b)
			} else {
				s := p.sessionMgr.Get(id)
				s.Write(b.Data())
			}
		} else {
			p.seq, p.ack = 0, 0
			// todo: attach ip hdr
			p.srv.st.Inbound(b)
		}
	}
}

func (p *proxyer) Send(b *relraw.Packet, id session.ID) {
	if debug.Debug() {
		require.True(test.T(), p.inited)
	}

	session.SetID(b, id)
	p.fake.SendAttach(b)
	p.crypto.Encrypt(b)

	p.raw.WriteCtx(p.ctx, b)
}

func (p *proxyer) MTU() int { return p.raw.MTU() }
