//go:build windows
// +build windows

package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/divert-go"
	sconn "github.com/lysShub/fatcp"
	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/client/capture"
	"github.com/lysShub/fatun/client/filter"
	"github.com/lysShub/fatun/control"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
	"github.com/lysShub/rawsock"
	"github.com/lysShub/rawsock/tcp"
	dconn "github.com/lysShub/rawsock/tcp/divert"
	"github.com/lysShub/rawsock/test"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/pkg/errors"
)

type Client struct {
	config *fatun.Config
	self   fatun.Session

	conn           *sconn.Conn
	divertPriority int16

	hiter         filter.Hitter
	filter.Filter // todo: if Client closed, Filter operate should return error
	capture       capture.Capture
	inject        *Inject
	ctr           control.Client

	pcap      atomic.Pointer[pcap.BindPcap]
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

	wraw, err := test.WrapPcap(raw, fmt.Sprintf("client-raw-%s.pcap", time.Now().Format("2006-01-02T15")))
	if err != nil {
		panic(err)
	}

	c, err := NewClient(ctx, wraw, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewClient(ctx context.Context, raw rawsock.RawConn, cfg *fatun.Config) (*Client, error) {
	var c = &Client{
		config: cfg,
		self: fatun.Session{
			Src:   raw.LocalAddr(),
			Proto: header.TCPProtocolNumber,
			Dst:   raw.RemoteAddr(),
		},
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	c.config.Logger.Info("dialing")
	var err error
	if c.conn, err = sconn.DialCtx(ctx, raw, cfg.Config); err != nil {
		return nil, c.close(err)
	} else {
		if /* dc */ _, ok := raw.(*dconn.Conn); ok {
			c.divertPriority = 2 // todo: dc.Priority()
		}
	}
	c.config.Logger.Info("connected", slog.String("proxy-server", c.conn.RemoteAddr().String()))

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
	tcp, err := c.conn.TCP(ctx)
	if err != nil {
		return nil, c.close(err)
	}
	c.ctr = control.NewClient(tcp)

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
				c.config.Logger.Warn(errors.WithMessage(cause, "session close").Error())
			} else {
				c.config.Logger.Error(cause.Error(), errorx.Trace(errors.WithStack(cause)))
			}
			c.closeErr.Store(&cause)
		}
		return cause
	}
	return *c.closeErr.Load()
}

func (c *Client) downlinkService() error {
	var (
		pkt = packet.Make(0, c.config.MaxRecvBuffSize)
	)

	for {
		id, err := c.conn.Recv(c.srvCtx, pkt.SetHead(0))
		if err != nil {
			if errorx.Temporary(err) {
				c.config.Logger.Warn(err.Error())
				continue
			} else {
				return c.close(err)
			}
		}
		if p := c.pcap.Load(); p != nil {

			switch id.Proto {
			case header.UDPProtocolNumber:
				udp := header.UDP(pkt.Bytes())
				if int(udp.Length()) != len(udp) {
					panic("abcdefg")
				}
			}

			if err := p.Inbound(id.Remote, id.Proto, pkt.Bytes()); err != nil {
				panic(err)
			}
		}

		err = c.inject.Inject(pkt, id)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) uplink(ctx context.Context, pkt *packet.Packet, id sconn.Peer) error {
	return c.conn.Send(ctx, pkt, id)
}

func (c *Client) AddPcap(file string) error {
	p, err := pcap.File(file)
	if err != nil {
		return err
	}

	b, err := pcap.Bind(p, c.self.Src.Addr())
	if err != nil {
		return err
	}

	if !c.pcap.CompareAndSwap(nil, b) {
		b.Close()
		return errors.New("one pcap already exist")
	}
	return nil
}

func (c *Client) DelPcap() error {
	p := c.pcap.Swap(nil)
	if p != nil {
		p.Close()
	}
	return nil
}

func (c *Client) Close() error { return c.close(nil) }
