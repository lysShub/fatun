package client

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"github.com/lysShub/relraw"
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

	ctrRaw control.CtrInject
	ctr    *control.Client
}

func NewClient(ctx context.Context, raw relraw.RawConn, cfg *Config) (*Client, error) {
	var c = &Client{
		ctx:  cctx.WithContext(ctx),
		cfg:  cfg,
		addr: raw.LocalAddrPort(),
	}

	if err := c.connect(raw); err != nil {
		return nil, err
	}

	go c.downlink()
	return c, nil
}

func (c *Client) connect(raw relraw.RawConn) error {
	ctx := cctx.WithTimeout(c.ctx, time.Second*10) // todo: from cfg
	defer ctx.Cancel(nil)

	conn := itun.WrapRawConn(raw, c.cfg.MTU)

	c.conn = sconn.Connect(ctx, conn, &c.cfg.Sconn)
	if err := ctx.Err(); err != nil {
		return err
	}

	c.ctr, c.ctrRaw = control.NewClient(ctx, time.Hour, c.conn)
	if err := ctx.Err(); err != nil {
		return err
	}

	go c.downlink()

	// todo: client 需要传入ctx
	// init config
	c.ctr.EndConfig()

	return nil
}

func (c *Client) AddProxy(s itun.Session) error {
	if !s.IsValid() {
		return itun.ErrInvalidSession(s)
	} else if s.SrcAddr.Addr() != c.addr.Addr() {
		return fmt.Errorf("client %s can't proxy ip %s", c.addr.Addr(), s.SrcAddr.Addr())
	} else if s.SrcAddr.Port() == c.addr.Port() {
		return fmt.Errorf("can't proxy self")
	}

	switch s.Proto {
	case itun.TCP:
		id, err := c.ctr.AddTCP(s.DstAddr)
		if err != nil {
			return err
		}
		return c.sessionMgr.Add(s, id)
	default:
		panic("impossible")
	}
}

func (c *Client) downlink() {
	n := c.conn.Raw().MTU()

	var seg = segment.NewSegment(n)
	for {
		seg.Packet().Sets(0, n)

		if err := c.conn.RecvSeg(c.ctx, seg); err != nil {
			c.ctx.Cancel(err)
			return
		}

		if id := seg.ID(); id == segment.CtrSegID {
			c.ctrRaw.Inject(seg)
		} else {
			s := c.sessionMgr.Get(id)
			if s != nil {
				s.Inject(seg)
			} else {
				fmt.Println("回复了没有注册的")
			}
		}
	}
}

func (c *Client) Close() error {
	return nil
}
