package client

import (
	"context"
	"errors"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"github.com/lysShub/relraw"
	pkge "github.com/pkg/errors"
)

type Config struct {
	Sconn sconn.Client
	MTU   uint16
}

type Client struct {
	ctx  cctx.CancelCtx
	cfg  *Config
	addr netip.AddrPort

	sessionMgr *SessionMgr

	conn *sconn.Conn

	c control.Client
}

func NewClient(ctx context.Context, raw relraw.RawConn, cfg *Config) (*Client, error) {
	var c = &Client{
		ctx:  cctx.WithContext(ctx),
		cfg:  cfg,
		addr: raw.LocalAddrPort(),
	}

	conn := itun.WrapRawConn(raw, c.cfg.MTU)
	c.conn = sconn.Connect(c.ctx, conn, &c.cfg.Sconn)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ctr, err := control.NewController(conn.LocalAddrPort(), conn.RemoteAddrPort(), conn.MTU())
	if err != nil {
		return nil, err
	}

	go ctr.OutboundService(c.ctx, c.conn)
	go c.downlink(ctr)

	c.c = control.Dial(c.ctx, ctr)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return c, nil
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
		resp, err := c.c.AddTCP(s.DstAddr)
		if err != nil {
			return err
		} else if resp.Err != nil {
			panic(err)
		}
		return c.sessionMgr.Add(s, resp.ID)
	default:
		panic("impossible")
	}
}

func (c *Client) downlink(ctrSessionInbound *control.Controller) {
	n := c.conn.Raw().MTU()

	var seg = segment.NewSegment(n)
	for {
		seg.Packet().Sets(0, n)

		if err := c.conn.RecvSeg(c.ctx, seg); err != nil {
			c.ctx.Cancel(err)
			return
		}

		if id := seg.ID(); id == segment.CtrSegID {
			ctrSessionInbound.Inbound(seg)
		} else {
			s := c.sessionMgr.Get(id)
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
		c.c.Close(),
		c.conn.Close(),
	)

	return err
}
