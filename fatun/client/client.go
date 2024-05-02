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
	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/client/capture"
	"github.com/lysShub/fatun/fatun/client/filter"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/conn"
	"github.com/lysShub/sockit/conn/tcp"
	dconn "github.com/lysShub/sockit/conn/tcp/divert"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/pkg/errors"
)

type Client struct {
	cfg  *fatun.Config
	self session.Session

	conn           *sconn.Conn
	divertPriority int16

	hiter filter.Hitter
	filter.Filter
	capture capture.Capture
	inject  *Inject
	ctr     control.Client

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

var _ = divert.MustLoad(divert.DLL)

func Proxy(ctx context.Context, server string, cfg *fatun.Config) (*Client, error) {
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

	// wraw, err := test.WrapPcap(raw, "client-raw.pcap")
	// if err != nil {
	// 	panic(err)
	// }

	c, err := NewClient(ctx, raw, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewClient(ctx context.Context, raw conn.RawConn, cfg *fatun.Config) (*Client, error) {
	var c = &Client{
		cfg: cfg,
		self: session.Session{
			Src:   raw.LocalAddr(),
			Proto: header.TCPProtocolNumber,
			Dst:   raw.RemoteAddr(),
		},
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	c.cfg.Logger.Info("dialing")
	var err error
	if c.conn, err = sconn.DialCtx(ctx, raw, cfg.Config); err != nil {
		return nil, c.close(err)
	} else {
		if /* dc */ _, ok := raw.(*dconn.Conn); ok {
			c.divertPriority = 2 // todo: dc.Priority()
		}
	}
	c.cfg.Logger.Info("connected", slog.String("proxy-server", c.conn.RemoteAddr().String()))

	if f, err := filter.New(); err != nil {
		return nil, c.close(err)
	} else {
		c.hiter, c.Filter = f, f
	}
	if c.capture, err = capture.New(captureImplPtr(c)); err != nil {
		return nil, c.close(err)
	}

	if c.inject, err = NewInject(test.LocIP()); err != nil {
		return nil, c.close(err)
	}

	go c.downlinkService()
	c.ctr = control.NewClient(c.conn.TCP())

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

		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			if errorx.Temporary(cause) {
				c.cfg.Logger.Warn(errors.WithMessage(cause, "session close").Error())
			} else {
				c.cfg.Logger.Error(cause.Error(), errorx.TraceAttr(errors.WithStack(cause)))
			}
			c.closeErr.Store(&cause)
		}
		return cause
	}
	return *c.closeErr.Load()
}

func (c *Client) downlinkService() error {
	var (
		tcp = packet.Make(32, c.cfg.MTU)
	)

	for {
		id, err := c.conn.Recv(c.srvCtx, tcp.Sets(32, 0xfff))
		if err != nil {
			if errorx.Temporary(err) {
				c.cfg.Logger.Warn(err.Error())
				continue
			} else {
				return c.close(err)
			}
		}

		err = c.inject.Inject(tcp, id)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) uplink(ctx context.Context, pkt *packet.Packet, id session.ID) error {
	return c.conn.Send(ctx, pkt, id)
}

func (c *Client) Close() error { return c.close(nil) }
