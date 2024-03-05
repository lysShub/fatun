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
	var err error
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
	go c.downlinkService()
	go c.uplinkService()

	tcp, err := gonet.DialTCPWithBind(
		c.ctx, c.st,
		raw.LocalAddrPort(), raw.RemoteAddrPort(),
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return nil, err
	}

	cfg.PrevPackets.Client(c.ctx, tcp)
	if c.ctx.Err() != nil {
		return nil, err
	}

	key, err := cfg.Config.SwapKey.SecretKey(c.ctx, tcp)
	if err != nil {
		return nil, err
	}
	c.crypto, err = crypto.NewTCP(key, c.pseudoSum1)
	if err != nil {
		return nil, err
	}
	c.fake = faketcp.NewFakeTCP(
		raw.LocalAddrPort().Port(),
		raw.RemoteAddrPort().Port(),
		c.seq, c.ack,
		&c.pseudoSum1,
	)

	c.inited.CompareAndSwap(false, true)

	c.ctr = control.NewClient(c.ctx, tcp)

	c.ctr.IPv6()

	defer func() { fmt.Println("client return") }()

	return c, c.ctr.EndConfig()
}

func (c *Client) Uplink(b *relraw.Packet, id session.ID) {
	session.SetID(b, id)

	c.fake.SendAttach(b)

	c.crypto.Encrypt(b)

	c.raw.WriteCtx(context.Background(), b)
}

func (c *Client) downlinkService() {
	mtu := c.MTU()
	var b = relraw.NewPacket(0, mtu)

	for {
		b.Sets(0, mtu)

		if err := c.raw.ReadCtx(c.ctx, b); err != nil {
			c.ctx.Cancel(err)
			return
		}

		if faketcp.IsFakeTCP(b.Data()) {
			if !c.inited.Load() {
				time.Sleep(time.Millisecond * 100)
			}

			err := c.crypto.Decrypt(b)
			if err != nil {
				if debug.Debug() {
					require.NoError(test.T(), err)
				}
				continue
			}

			c.fake.RecvStrip(b)

			id := session.GetID(b)
			if id == session.CtrSessID {
				c.ipstack.AttachInbound(b)
				c.st.Inbound(b)
			} else {
				s := c.sessionMgr.Get(id)
				s.Inject(b)
			}
		} else {
			c.ack = max(c.ack, header.TCP(b.Data()).AckNumber())

			{
				tcp := header.TCP(b.Data())
				if tcp.Flags().Contains(header.TCPFlagPsh) {
					fmt.Println("recv", string(tcp.Payload()))
				}
			}

			c.ipstack.AttachInbound(b)
			c.st.Inbound(b)
		}
	}
}

func (c *Client) uplinkService() {
	mtu := c.MTU()
	var b = relraw.NewPacket(0, mtu)

	for {
		b.Sets(0, mtu)
		c.st.Outbound(c.ctx, b)

		if c.inited.Load() {
			c.Uplink(b, session.CtrSessID)
		} else {
			c.seq = max(c.seq, header.TCP(b.Data()).SequenceNumber())

			// recover to ip packet
			b.SetHead(0)
			c.raw.Write(b.Data())
		}
	}
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
		resp, err := c.ctr.AddTCP(s.DstAddr)
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
