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

func Proxy(c context.Context, srv *Server, raw *itun.RawConn) {
	p, err := NewProxyer(c, srv, raw)
	if err != nil {
		panic(err)
	}

	err = p.HandShake()
	if err != nil {
		panic(err)
	}
}

type Proxyer struct {
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

	ctr             control.Server
	endConfigNotify chan struct{}
}

var _ Downlink = (*Proxyer)(nil)

func NewProxyer(c context.Context, srv *Server, raw *itun.RawConn) (*Proxyer, error) {
	var p = &Proxyer{
		ctx: cctx.WithContext(c),
		srv: srv,
		raw: raw,

		pseudoSum1: header.PseudoHeaderChecksum(
			header.TCPProtocolNumber,
			raw.LocalAddr().Addr, raw.RemoteAddr().Addr,
			0,
		),

		initNotify:      make(chan struct{}),
		endConfigNotify: make(chan struct{}),
	}
	p.sessionMgr = NewSessionMgr(p)

	var err error
	if p.ipstack, err = relraw.NewIPStack(
		raw.LocalAddrPort().Addr(),
		raw.RemoteAddrPort().Addr(),
		header.TCPProtocolNumber,
	); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Proxyer) HandShake() error {
	go p.uplinkService()
	go p.downlinkService()

	tcp, err := p.srv.ctrListener.AcceptBy(p.ctx, p.raw.RemoteAddrPort())
	if err != nil {
		return err
	}

	err = p.srv.cfg.PrevPackets.Server(p.ctx, tcp)
	if err != nil {
		return err
	}

	key, err := p.srv.cfg.SwapKey.SecretKey(p.ctx, tcp)
	if err != nil {
		return err
	}

	p.crypto, err = crypto.NewTCP(key, p.pseudoSum1)
	if err != nil {
		return err
	}

	// todo: NewFakeTCP not need calc csum
	p.fake = faketcp.NewFakeTCP(
		p.raw.LocalAddr().Port,
		p.raw.RemoteAddr().Port,
		p.seq, p.ack, &p.pseudoSum1,
	)

	// wait init: when recve first fakt tcp packet
	p.prepareInit.CompareAndSwap(false, true)
	<-p.initNotify
	p.inited.CompareAndSwap(false, true)

	p.ctr = control.NewServer(tcp, proxyerImplPtr(p))

	go p.controlService()
	<-p.endConfigNotify
	return nil
}

func (p *Proxyer) downlinkService() {
	dst := p.raw.RemoteAddrPort()
	mtu := p.raw.MTU()

	var tcp = relraw.NewPacket(0, mtu)
	for {
		tcp.SetHead(0)
		p.srv.st.OutboundBy(p.ctx, dst, tcp)

		if p.inited.Load() {
			p.downlink(tcp, session.CtrSessID)
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

func (p *Proxyer) uplinkService() {
	var (
		mtu     = p.raw.MTU()
		minSize = header.TCPMinimumSize + session.Size

		seg = relraw.NewPacket(0, mtu)
	)

	for {
		seg.Sets(0, mtu)

		err := p.raw.ReadCtx(p.ctx, seg)
		if err != nil {
			p.ctx.Cancel(err)
			return
		} else if seg.Len() < minSize {
			continue
		}

		if faketcp.IsFakeTCP(seg.Data()) {
			if p.prepareInit.CompareAndSwap(true, false) {
				close(p.initNotify)
			}

			err = p.crypto.Decrypt(seg)
			if err != nil {
				p.ctx.Cancel(err)
				return
			}

			p.fake.RecvStrip(seg)

			id := session.GetID(seg)
			if id == session.CtrSessID {
				p.ipstack.AttachInbound(seg)
				if debug.Debug() {
					test.ValidIP(test.T(), seg.Data())
				}

				p.srv.st.Inbound(seg)
			} else {
				s, err := p.sessionMgr.Get(id)
				if err != nil {
					fmt.Println(err)
				} else {
					s.Send(seg)
				}
			}
		} else {
			p.ack = max(p.ack, header.TCP(seg.Data()).AckNumber())

			p.ipstack.AttachInbound(seg)
			if debug.Debug() {
				test.ValidIP(test.T(), seg.Data())
			}
			p.srv.st.Inbound(seg)
		}
	}
}

func (p *Proxyer) controlService() {
	err := p.ctr.Serve(p.ctx)
	if err != nil {
		p.ctx.Cancel(err)
	}
}

func (p *Proxyer) downlink(pkt *relraw.Packet, id session.ID) error {
	if debug.Debug() {
		require.True(test.T(), p.inited.Load())
	}

	session.SetID(pkt, id)

	p.fake.SendAttach(pkt)

	p.crypto.Encrypt(pkt)
	if debug.Debug() {
		test.ValidTCP(test.T(), pkt.Data(), p.pseudoSum1)
	}

	err := p.raw.WriteCtx(p.ctx, pkt)
	return err
}

func (p *Proxyer) MTU() int { return p.raw.MTU() }
