//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type proxyer struct {
	ctx cctx.CancelCtx
	srv *Server
	raw *itun.RawConn

	sessionMgr *SessionMgr

	pseudoSum1 uint16
	ipstack    *relraw.IPStack
	seq, ack   uint32

	prepareInit atomic.Bool
	initNotify  chan struct{}
	inited      atomic.Bool

	fake   *faketcp.FakeTCP
	crypto *crypto.TCP
}

var _ Downlink = (*proxyer)(nil)

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	var err error

	var p = &proxyer{
		ctx: cctx.WithContext(c),
		srv: srv,
		raw: raw,

		pseudoSum1: header.PseudoHeaderChecksum(
			header.TCPProtocolNumber,
			raw.LocalAddr().Addr, raw.RemoteAddr().Addr,
			0,
		),

		initNotify: make(chan struct{}),
	}
	p.sessionMgr = NewSessionMgr(p)
	if p.ipstack, err = relraw.NewIPStack(
		raw.LocalAddrPort().Addr(),
		raw.RemoteAddrPort().Addr(),
		header.TCPProtocolNumber,
	); err != nil {
		panic(err)
	}
	go p.uplinkService()
	go p.downlinkService()

	tcp, err := p.srv.ctrListener.AcceptBy(p.ctx, raw.RemoteAddrPort())
	if err != nil {
		panic(err)
	}

	p.srv.cfg.PrevPackets.Server(p.ctx, tcp)
	if p.ctx.Err() != nil {
		panic(p.ctx.Err())
	}

	key, err := p.srv.cfg.SwapKey.SecretKey(p.ctx, tcp)
	if err != nil {
		panic(err)
	}

	p.crypto, err = crypto.NewTCP(key, p.pseudoSum1)
	if err != nil {
		panic(err)
	}
	p.fake = faketcp.NewFakeTCP(
		p.raw.LocalAddr().Port,
		p.raw.RemoteAddr().Port,
		p.seq, p.ack, &p.pseudoSum1,
	)

	// wait init: when recve first fakt tcp packet
	p.prepareInit.CompareAndSwap(false, true)
	<-p.initNotify
	p.inited.CompareAndSwap(false, true)

	control.Serve(p.ctx, tcp, proxyerImplPtr(p))

	<-p.ctx.Done()
	e := p.ctx.Err()
	fmt.Println("proxy closed: ", e)
}

func (p *proxyer) downlinkService() {
	dst := p.raw.RemoteAddrPort()
	mtu := p.raw.MTU()

	var tcp = relraw.NewPacket(0, mtu)
	for {
		tcp.SetHead(0)
		p.srv.st.OutboundBy(p.ctx, dst, tcp)

		if p.inited.Load() {
			p.Downlink(tcp, session.CtrSessID)
		} else {
			p.seq = max(p.seq, header.TCP(tcp.Data()).SequenceNumber())

			// recover to ip packet
			tcp.SetHead(0)
			if debug.Debug() {
				test.ValidIP(test.T(), tcp.Data())
			}
			_, err := p.raw.Write(tcp.Data())
			if err != nil {
				p.ctx.Cancel(err)
				return
			}
		}
	}
}

func (p *proxyer) uplinkService() {
	var (
		mtu     = p.raw.MTU()
		minSize = header.TCPMinimumSize + session.Size

		b = relraw.NewPacket(0, mtu)
	)

	for {
		b.Sets(0, mtu)

		err := p.raw.ReadCtx(p.ctx, b)
		if err != nil {
			p.ctx.Cancel(err)
			return
		} else if b.Len() < minSize {
			continue
		}

		if faketcp.IsFakeTCP(b.Data()) {
			if p.prepareInit.CompareAndSwap(true, false) {
				close(p.initNotify)
			}

			err = p.crypto.Decrypt(b)
			if err != nil {
				p.ctx.Cancel(err)
				return
			}

			p.fake.RecvStrip(b)

			id := session.GetID(b)
			if id == session.CtrSessID {
				p.ipstack.AttachInbound(b)
				if debug.Debug() {
					test.ValidIP(test.T(), b.Data())
				}

				p.srv.st.Inbound(b)
			} else {
				s := p.sessionMgr.Get(id)
				s.Send(b.Data())
			}
		} else {
			p.ack = max(p.ack, header.TCP(b.Data()).AckNumber())

			p.ipstack.AttachInbound(b)
			if debug.Debug() {
				test.ValidIP(test.T(), b.Data())
			}
			p.srv.st.Inbound(b)
		}
	}
}

func (p *proxyer) Downlink(b *relraw.Packet, id session.ID) {
	if debug.Debug() {
		require.True(test.T(), p.inited.Load())
	}

	session.SetID(b, id)
	p.fake.SendAttach(b)
	p.crypto.Encrypt(b)

	p.raw.WriteCtx(p.ctx, b)
}

func (p *proxyer) MTU() int { return p.raw.MTU() }
