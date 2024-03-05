package client

import (
	"context"
	"errors"
	"net/netip"

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
	Secret crypto.SecretKey

	MTU  uint16
	IPv6 bool
}

type Client struct {
	ctx cctx.CancelCtx
	cfg *Config

	raw  *itun.RawConn
	addr netip.AddrPort

	sessionMgr *SessionMgr

	seq, ack uint32

	inited bool
	st     *ustack.Ustack

	fake   *faketcp.FakeTCP
	crypto *crypto.TCP

	ctr control.Client
}

var _ Uplink = (*Client)(nil)

func NewClient(parentCtx context.Context, raw relraw.RawConn, cfg *Config) (*Client, error) {
	var c = &Client{
		ctx:  cctx.WithContext(parentCtx),
		cfg:  cfg,
		addr: raw.LocalAddrPort(),
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

	key, err := cfg.Secret.SecretKey(c.ctx, tcp)
	if err != nil {
		return nil, err
	}
	c.crypto, err = crypto.NewTCP(key, 0) // todo:
	if err != nil {
		return nil, err
	}

	c.fake = faketcp.NewFakeTCP(
		raw.LocalAddrPort().Port(),
		raw.RemoteAddrPort().Port(),
		c.seq, c.ack,
		nil, // todo:
	)

	c.inited = true

	c.ctr = control.NewClient(c.ctx, tcp)

	c.ctr.IPv6()

	return c, c.ctr.EndConfig()
}

func (c *Client) AddProxy(s session.Session) error {
	if !s.IsValid() {
		return session.ErrInvalidSession(s)
	} else if s.SrcAddr.Addr() != c.addr.Addr() {
		return pkge.Errorf("client %s can't proxy ip %s", c.addr.Addr(), s.SrcAddr.Addr())
	} else if s.SrcAddr.Port() == c.addr.Port() {
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

func (c *Client) Uplink(b *relraw.Packet, id session.ID) {
	session.SetID(b, id)

	c.fake.SendAttach(b)

	c.crypto.Encrypt(b)

	c.raw.WriteCtx(context.Background(), b)
}

func (c *Client) MTU() int { return c.raw.MTU() }

func (c *Client) downlinkService() {
	mtu := c.MTU()
	var p = relraw.NewPacket(0, mtu)

	for {
		p.Sets(0, mtu)

		if err := c.raw.ReadCtx(c.ctx, p); err != nil {
			c.ctx.Cancel(err)
			return
		}

		if faketcp.IsFakeTCP(p.Data()) {
			err := c.crypto.Decrypt(p)
			if err != nil {
				if debug.Debug() {
					require.NoError(test.T(), err)
				}
				continue
			}

			c.fake.RecvStrip(p)

			id := session.GetID(p)
			if id == session.CtrSessID {
				c.st.Inbound(p)
			} else {
				s := c.sessionMgr.Get(id)
				s.Inject(p)
			}
		} else {
			c.ack = max(c.ack, header.TCP(p.Data()).AckNumber())

			// recover to ip packet
			p.SetHead(0)
			c.st.Inbound(p)
		}
	}
}

func (c *Client) uplinkService() {
	mtu := c.MTU()
	var b = relraw.NewPacket(0, mtu)

	for {
		b.Sets(0, mtu)
		c.st.Outbound(c.ctx, b)

		if c.inited {
			c.Uplink(b, session.CtrSessID)
		} else {
			c.seq = max(c.seq, header.TCP(b.Data()).SequenceNumber())

			// recover to ip packet
			b.SetHead(0)
			c.raw.Write(b.Data())
		}
	}
}

func (c *Client) Close() error {
	err := errors.Join(
		c.ctr.Close(),
	)

	return err
}
