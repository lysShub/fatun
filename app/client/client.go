package client

import (
	"context"
	"errors"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
	pkge "github.com/pkg/errors"
)

type Config struct {
	Sconn sconn.Config
	MTU   uint16
	IPv6  bool
}

type Client struct {
	ctx  cctx.CancelCtx
	cfg  *Config
	addr netip.AddrPort

	sessionMgr *SessionMgr

	conn *sconn.Conn

	ctr control.Client
}

func NewClient(parentCtx context.Context, raw relraw.RawConn, cfg *Config) (*Client, error) {
	var c = &Client{
		ctx:  cctx.WithContext(parentCtx),
		cfg:  cfg,
		addr: raw.LocalAddrPort(),
	}

	conn := itun.WrapRawConn(raw, c.cfg.MTU)
	c.conn = sconn.Connect(c.ctx, conn, &c.cfg.Sconn)
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}

	ctr, err := control.NewController(conn.LocalAddrPort(), conn.RemoteAddrPort(), conn.MTU())
	if err != nil {
		return nil, err
	}

	go ctr.OutboundService(c.ctx, c.conn)
	go c.downlink(ctr)

	c.ctr = control.Dial(c.ctx, ctr)
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}

	if cfg.IPv6, err = c.ctr.IPv6(); err != nil {
		return nil, errors.Join(err, c.Close())
	}

	return c, c.ctr.EndConfig()
}

func (c *Client) AddProxy(s itun.Session) error {
	if !s.IsValid() {
		return itun.ErrInvalidSession(s)
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

func (c *Client) downlink(ctrSessionInbound *control.Controller) {
	n := c.conn.Raw().MTU()

	var seg = relraw.NewPacket(0, n)
	for {
		seg.Sets(0, n)

		id, err := c.conn.RecvSeg(c.ctx, seg)
		if err != nil {
			c.ctx.Cancel(err)
			return
		}

		if id == session.CtrSessID {
			ctrSessionInbound.Inbound(seg)
		} else {
			s := c.sessionMgr.Get(uint16(id))
			if s != nil {
				s.Inject(seg)
			} else {
				// todo: log reply without registed
			}
		}
	}
}

func (c *Client) Close() error {
	err := errors.Join(
		c.ctr.Close(),
		c.conn.Close(),
	)

	return err
}
