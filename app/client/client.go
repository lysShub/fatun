//go:build windows
// +build windows

package client

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/app/client/capture"
	"github.com/lysShub/fatun/app/client/filter"
	cs "github.com/lysShub/fatun/app/client/session"
	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/conn"
	"github.com/lysShub/sockit/conn/tcp"
	dconn "github.com/lysShub/sockit/conn/tcp/divert"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/pkg/errors"
)

type Client struct {
	cfg    *app.Config
	logger *slog.Logger
	self   session.Session

	conn           *sconn.Conn
	divertPriority int16

	sessMgr *cs.SessionMgr
	hiter   filter.Hitter
	filter.Filter
	capture capture.Capture

	ctr control.Client

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

var _ = divert.MustLoad(divert.DLL)

func Proxy(ctx context.Context, server string, cfg *app.Config) (*Client, error) {
	var laddr, raddr netip.AddrPort
	if addr, err := net.ResolveTCPAddr("tcp", server); err != nil {
		return nil, errors.WithStack(err)
	} else {
		ip := addr.IP
		if ip == nil {
			laddr = netip.AddrPortFrom(netip.IPv4Unspecified(), 0)
			raddr = netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(addr.Port))
		} else if ip.To4() != nil {
			laddr = netip.AddrPortFrom(netip.IPv4Unspecified(), 0)
			raddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ip.To4())), uint16(addr.Port))
		} else {
			laddr = netip.AddrPortFrom(netip.IPv6Unspecified(), 0)
			raddr = netip.AddrPortFrom(netip.AddrFrom16([16]byte(ip.To16())), uint16(addr.Port))
		}
	}

	raw, err := tcp.Connect(laddr, raddr)
	if err != nil {
		return nil, err
	}

	// wraw, err := test.WrapPcap(raw, "client.pcap")
	// if err != nil {
	// 	panic(err)
	// }

	c, err := NewClient(ctx, raw, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewClient(ctx context.Context, raw conn.RawConn, cfg *app.Config) (*Client, error) {
	var c = &Client{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("client").WithAttrs([]slog.Attr{
			{Key: "local", Value: slog.StringValue(raw.LocalAddr().String())},
			{Key: "proxyer", Value: slog.StringValue(raw.RemoteAddr().String())},
		})),
		self: session.Session{
			Src:   raw.LocalAddr(),
			Proto: header.TCPProtocolNumber,
			Dst:   raw.RemoteAddr(),
		},
		sessMgr: cs.NewSessionMgr(),
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	var err error
	if c.conn, err = sconn.Dial(raw, cfg.Config); err != nil {
		return nil, c.close(err)
	} else {
		if /* dc */ _, ok := raw.(*dconn.Conn); ok {
			c.divertPriority = 2 // todo: dc.Priority()
		}
	}
	c.logger.Info("connected server")

	if f, err := filter.New(); err != nil {
		return nil, c.close(err)
	} else {
		c.hiter, c.Filter = f, f
	}
	if c.capture, err = capture.New(captureImplPtr(c)); err != nil {
		return nil, c.close(err)
	}

	go c.downlinkService()
	c.ctr = control.NewClient(c.conn.TCP())

	// todo: init config
	if err := c.ctr.InitConfig(ctx, &control.Config{}); err != nil {
		return nil, c.close(err)
	}
	return c, nil
}

func (c *Client) close(cause error) error {
	if c.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if c.ctr != nil {
			if err := c.ctr.Close(); err != nil {
				cause = err
			}
		}
		c.srvCancel()
		if c.sessMgr != nil {
			if err := c.sessMgr.Close(); err != nil {
				cause = err
			}
		}
		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			c.closeErr.Store(&cause)
			c.logger.Info("close", "cause", cause.Error(), errorx.TraceAttr(errors.WithStack(cause)))
		}
		return cause
	}
	return *c.closeErr.Load()
}

func (c *Client) downlinkService() error {
	var (
		tcp = packet.Make(64, c.cfg.MTU)
	)

	for {
		id, err := c.conn.Recv(c.srvCtx, tcp.SetHead(64))
		if err != nil {
			if errorx.Temporary(err) {
				c.logger.Warn(err.Error())
			} else {
				return c.close(err)
			}
		}

		s, err := c.sessMgr.Get(id)
		if err != nil {
			c.logger.Warn(err.Error())
			continue
		}

		err = s.Inject(tcp)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) uplink(ctx context.Context, pkt *packet.Packet, id session.ID) error {
	return c.conn.Send(ctx, pkt, id)
}

func (c *Client) Close() error { return c.close(nil) }
