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
)

type Config struct {
	sconn.Config
	MTU uint16
}

type Client struct {
	ctx  cctx.CancelCtx
	cfg  *Config
	addr netip.AddrPort

	sessionMgr *SessionMgr

	conn *sconn.Conn

	// todo: merge
	ctrConn *control.CtrConn
	ctr     *control.Client
}

func NewClient(ctx context.Context, localAddr, pxySrvAddr netip.AddrPort, cfg *Config) (*Client, error) {
	var c = &Client{
		ctx: cctx.WithContext(ctx),
		cfg: cfg,
	}

	if err := c.Connect(pxySrvAddr); err != nil {
		return nil, err
	}

	return nil, nil
}

func (c *Client) Connect(server netip.AddrPort) error {
	ctx := cctx.WithTimeout(c.ctx, time.Second*10) // todo: from cfg
	defer ctx.Cancel(nil)

	raw, err := connectRaw(server)
	if err != nil {
		return err
	}
	conn := itun.WrapRawConn(raw, c.cfg.MTU)

	c.conn = sconn.Connect(ctx, conn, &c.cfg.Config)
	if err := ctx.Err(); err != nil {
		return err
	}

	c.ctrConn = control.Connect(ctx, c.conn)
	if err := ctx.Err(); err != nil {
		return err
	}
	c.ctr = control.NewCtrClient(ctx, c.ctrConn)
	if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}

func (c *Client) AddProxy(s itun.Session) error {
	if !s.IsValid() {
		return itun.ErrInvalidSession(s)
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
			c.ctrConn.Inject(seg)
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

// var server netip.AddrPort
// if a, err := net.ResolveTCPAddr("tcp", pxySrvAddr); err != nil {
// 	return nil, err
// } else {
// 	if a.Port == 0 {
// 		a.Port = itun.DefaultPort
// 	}
// 	addr, ok := netip.AddrFromSlice(a.IP)
// 	if !ok {
// 		return nil, fmt.Errorf("invalid proxy server address %s", pxySrvAddr)
// 	} else if addr.Is4In6() {
// 		addr = netip.AddrFrom4(addr.As4())
// 	}
// 	server = netip.AddrPortFrom(addr, uint16(a.Port))
// }
