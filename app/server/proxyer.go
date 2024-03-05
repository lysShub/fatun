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
	ctx        cctx.CancelCtx
	srv        *Server
	sessionMgr *SessionMgr
	session    session.Session

	raw *itun.RawConn

	SrcAddr netip.AddrPort

	ipstack *relraw.IPStack

	inited bool

	seq, ack uint32

	pseudoSum1 uint16
	fake       *faketcp.FakeTCP
	crypto     *crypto.TCP
}

var _ Downlink = (*proxyer)(nil)

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	var p = &proxyer{
		ctx:     cctx.WithContext(c),
		srv:     srv,
		SrcAddr: raw.RemoteAddrPort(),

		raw: raw,
	}
	p.sessionMgr = NewSessionMgr(p)

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
	p.inited = true

	control.Serve(p.ctx, tcp, proxyerImplPtr(p))

	<-p.ctx.Done()
	e := p.ctx.Err()
	fmt.Println("proxy closed: ", e)
}

func (p *proxyer) downlinkService() {
	dst := p.raw.RemoteAddrPort()
	mtu := p.raw.MTU()
	b := relraw.NewPacket(0, mtu)

	p.srv.st.OutboundBy(p.ctx, dst, b)

	if p.inited {
		p.Downlink(b, session.CtrSessID)
	} else {
		p.seq = max(p.seq, header.TCP(b.Data()).SequenceNumber())

		// recover to ip packet
		b.SetHead(0)
		if debug.Debug() {
			test.ValidIP(test.T(), b.Data())
		}
		_, err := p.raw.Write(b.Data())
		if err != nil {
			p.ctx.Cancel(err)
			return
		}
	}
}

type ErrManyDecryptFailPacket string

func (e ErrManyDecryptFailPacket) Error() string {
	return fmt.Sprintf("recved many decrypt fail segment, %s", string(e))
}

func (p *proxyer) uplinkService() {
	var (
		mtu     = p.raw.MTU()
		minSize = p.session.MinPacketSize()
		tinyCnt = uint8(0)

		b = relraw.NewPacket(0, mtu)
	)
	const tinyCntLimit = 4 // todo: to config

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
			err = p.crypto.Decrypt(b)
			if err != nil {
				if debug.Debug() {
					require.NoError(test.T(), err)
				}

				tinyCnt++
				if tinyCnt > tinyCntLimit {
					p.ctx.Cancel(ErrManyDecryptFailPacket(err.Error()))
					return
				} else {
					continue
				}
			} else {
				tinyCnt = 0
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
		require.True(test.T(), p.inited)
	}

	session.SetID(b, id)
	p.fake.SendAttach(b)
	p.crypto.Encrypt(b)

	p.raw.WriteCtx(p.ctx, b)
}

func (p *proxyer) MTU() int { return p.raw.MTU() }
