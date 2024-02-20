package client

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
)

type Client struct {
	ctx        cctx.CancelCtx
	cfg        *config.Client
	sessionMgr *SessionMgr

	conn *sconn.Conn

	ctrConn *control.CtrConn
	ctr     *control.Client
}

func NewClient(proxyServer string, cfg *config.Client) (*Client, error) {
	var c = &Client{
		cfg: &config.Client{},
	}

	var server netip.AddrPort
	if a, err := net.ResolveTCPAddr("tcp", proxyServer); err != nil {
		return nil, err
	} else {
		if a.Port == 0 {
			a.Port = itun.DefaultPort
		}
		addr, ok := netip.AddrFromSlice(a.IP)
		if !ok {
			return nil, fmt.Errorf("invalid proxy server address %s", proxyServer)
		} else if addr.Is4In6() {
			addr = netip.AddrFrom4(addr.As4())
		}
		server = netip.AddrPortFrom(addr, uint16(a.Port))
	}

	ctx := cctx.WithContext(context.Background())

	if err := c.Connect(ctx, server); err != nil {
		return nil, err
	}

	return nil, nil
}

func (c *Client) Connect(ctx cctx.CancelCtx, server netip.AddrPort) error {
	raw, err := connectRaw(server)
	if err != nil {
		return err
	}
	conn := itun.WrapRawConn(raw, c.cfg.MTU)

	c.conn = sconn.Connect(ctx, conn, &c.cfg.Sconn)
	if err := ctx.Err(); err != nil {
		return err
	}

	c.ctrConn = control.ConnectCtrConn(ctx, c.conn)
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
