package client

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	pkge "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	config.Config

	MTU  uint16
	IPv6 bool
}

type Client struct {
	ctx cctx.CancelCtx
	cfg *Config
	raw *itun.RawConn

	sessionMgr *SessionMgr
	st         *ustack.Ustack

	pseudoSum1 uint16
	seq, ack   uint32
	ipstack    *relraw.IPStack

	inited atomic.Bool

	fake   *faketcp.FakeTCP
	crypto *crypto.TCP
	ctr    control.Client
}

var _ Uplink = (*Client)(nil)

func NewClient(parentCtx context.Context, raw relraw.RawConn, cfg *Config) (*Client, error) {
	var c = &Client{
		ctx: cctx.WithContext(parentCtx),
		cfg: cfg,
		raw: itun.WrapRawConn(raw, cfg.MTU),
	}
	c.sessionMgr = NewSessionMgr(c)
	c.pseudoSum1 = header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		c.raw.LocalAddr().Addr, c.raw.RemoteAddr().Addr,
		0,
	)

	var err error
	if c.st, err = ustack.NewUstack( // todo: set no delay
		c.raw.LocalAddrPort(),
		c.raw.MTU(),
	); err != nil {
		return nil, err
	}

	if c.ipstack, err = relraw.NewIPStack(
		c.raw.LocalAddrPort().Addr(), c.raw.RemoteAddrPort().Addr(),
		header.TCPProtocolNumber,
	); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Handshake() error {
	go c.downlinkService()
	go c.uplinkService()

	tcp, err := gonet.DialTCPWithBind(
		c.ctx, c.st,
		c.raw.LocalAddrPort(), c.raw.RemoteAddrPort(),
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return err
	}

	err = c.cfg.PrevPackets.Client(c.ctx, tcp)
	if err != nil {
		return err
	}

	key, err := c.cfg.Config.SwapKey.SecretKey(c.ctx, tcp)
	if err != nil {
		return err
	}
	c.crypto, err = crypto.NewTCP(key, c.pseudoSum1)
	if err != nil {
		return err
	}
	c.fake = faketcp.NewFakeTCP(
		c.raw.LocalAddrPort().Port(),
		c.raw.RemoteAddrPort().Port(),
		c.seq, c.ack,
		&c.pseudoSum1,
	)

	c.inited.CompareAndSwap(false, true)

	c.ctr = control.NewClient(tcp)

	c.ctr.IPv6(c.ctx)

	return c.ctr.EndConfig(c.ctx)
}

func (c *Client) downlinkService() {
	mtu := c.MTU()
	var tcp = relraw.NewPacket(0, mtu)

	for {
		tcp.Sets(0, mtu)

		if err := c.raw.ReadCtx(c.ctx, tcp); err != nil {
			c.ctx.Cancel(err)
			return
		}

		if faketcp.IsFakeTCP(tcp.Data()) {
			if !c.inited.Load() {
				time.Sleep(time.Millisecond * 100)
			}

			err := c.crypto.Decrypt(tcp)
			if err != nil {
				if debug.Debug() {
					require.NoError(test.T(), err)
				}
				continue
			}

			c.fake.RecvStrip(tcp)

			id := session.GetID(tcp)
			if id == session.CtrSessID {

				c.ipstack.AttachInbound(tcp)
				c.st.Inbound(tcp)
			} else {
				s, err := c.sessionMgr.Get(id)
				if err != nil {
					fmt.Println(err) // todo: log
					continue
				}

				err = s.Inject(tcp)
				if err != nil {
					c.sessionMgr.Del(id, err)
				}
			}
		} else {
			c.ack = max(c.ack, header.TCP(tcp.Data()).AckNumber())

			{
				tcp := header.TCP(tcp.Data())
				if tcp.Flags().Contains(header.TCPFlagPsh) {
					fmt.Println("recv", string(tcp.Payload()))
				}
			}

			c.ipstack.AttachInbound(tcp)
			c.st.Inbound(tcp)
		}
	}
}

func (c *Client) uplinkService() {
	mtu := c.MTU()
	var pkt = relraw.NewPacket(0, mtu)

	for {
		pkt.Sets(0, mtu)
		c.st.Outbound(c.ctx, pkt)

		if c.inited.Load() {
			c.uplink(pkt, session.CtrSessID)
		} else {
			c.seq = max(c.seq, header.TCP(pkt.Data()).SequenceNumber())

			// recover to ip packet
			pkt.SetHead(0)
			c.raw.Write(pkt.Data())
		}
	}
}

func (c *Client) uplink(pkt *relraw.Packet, id session.ID) error {
	if debug.Debug() {
		require.True(test.T(), c.inited.Load())
	}

	session.SetID(pkt, id)

	c.fake.SendAttach(pkt)

	c.crypto.Encrypt(pkt)

	err := c.raw.WriteCtx(context.Background(), pkt)
	return err
}

func (c *Client) MTU() int { return c.raw.MTU() }

func (c *Client) AddProxy(s session.Session) error {
	addr := c.raw.LocalAddrPort()
	if !s.IsValid() {
		return session.ErrInvalidSession(s)
	} else if s.SrcAddr.Addr() != addr.Addr() {
		return pkge.Errorf("client %s can't proxy ip %s", addr.Addr(), s.SrcAddr.Addr())
	} else if s.SrcAddr.Port() == addr.Port() {
		return pkge.Errorf("can't proxy self")
	}

	switch s.Proto {
	case itun.TCP:
		resp, err := c.ctr.AddTCP(c.ctx, s.DstAddr)
		if err != nil {
			return err
		} else if resp.Err != nil {
			panic(resp.Err)
		}
		return c.sessionMgr.Add(s, resp.ID)
	default:
		panic("impossible")
	}
}

func (c *Client) Close() error {
	err := errors.Join(
		c.ctr.Close(),
	)

	return err
}
