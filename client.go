package fatun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun/checksum"
	"github.com/lysShub/fatun/peer"
	"github.com/lysShub/rawsock/test"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	stdsum "gvisor.dev/gvisor/pkg/tcpip/checksum"
)

type Capture interface {
	Capture(ctx context.Context, ip *packet.Packet) error
	Inject(ctx context.Context, ip *packet.Packet) error
	Close() error
}

type Client[P peer.Peer] struct {
	Logger *slog.Logger

	Conn fatcp.Conn[P]

	Capture Capture

	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr errorx.CloseErr
}

func NewClient[P peer.Peer](opts ...func(*Client[P])) (*Client[P], error) {
	var c = &Client[P]{}
	for _, opt := range opts {
		opt(c)
	}

	if c.Logger == nil {
		c.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	if c.Conn == nil {
		return nil, errors.New("require fatcp.Conn")
	}
	var err error
	if c.Capture == nil {
		c.Capture, err = NewDefaultCapture(c.Conn.LocalAddr(), c.Conn.Overhead())
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Client[P]) Run(peer peer.Peer) (err error) {
	c.srvCtx, c.cancel = context.WithCancel(context.Background())
	go c.uplinkService()
	go c.downlinkServic()
	return nil
}

func (c *Client[P]) close(cause error) (_ error) {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)

		if c.cancel != nil {
			c.cancel()
		}
		if c.Capture != nil {
			errs = append(errs, c.Capture.Close())
		}
		if c.Conn != nil {
			errs = append(errs, c.Conn.Close())
		}
		return
	})
}

func (c *Client[P]) uplinkService() (_ error) {
	var (
		overhead = c.Conn.Overhead()
		ip       = packet.Make(c.Conn.MTU())
		peer     = (*new(P)).New().(P)
	)

	for {
		err := c.Capture.Capture(c.srvCtx, ip.Sets(overhead, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				c.Logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				return c.close(err)
			}
		}
		hdr := header.IPv4(ip.Bytes())
		peer.Reset(hdr.TransportProtocol(), netip.AddrFrom4(hdr.DestinationAddress().As4()))

		pkt := checksum.Client(ip)
		if err = c.Conn.Send(c.srvCtx, peer, pkt); err != nil {
			return c.close(err)
		}
	}
}

func (c *Client[P]) downlinkServic() error {
	var (
		pkt  = packet.Make(0, c.Conn.MTU())
		peer = (*new(P)).New().(P)
	)

	for {
		err := c.Conn.Recv(c.srvCtx, peer, pkt.Sets(0, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				c.Logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				return c.close(err)
			}
		}

		ip := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(pkt.Data()),
			TTL:         64,
			Protocol:    uint8(peer.Protocol()),
			SrcAddr:     tcpip.AddrFrom4(peer.Peer().As4()),
			DstAddr:     tcpip.AddrFrom4(c.Conn.LocalAddr().Addr().As4()),
		})
		rechecksum(ip)

		if err = c.Capture.Inject(c.srvCtx, pkt); err != nil {
			return c.close(err)
		}
	}
}

func (c *Client[P]) Close() error { return c.close(nil) }

func rechecksum(ip header.IPv4) {
	ip.SetChecksum(0)
	ip.SetChecksum(^ip.CalculateChecksum())

	psum := header.PseudoHeaderChecksum(
		ip.TransportProtocol(),
		ip.SourceAddress(),
		ip.DestinationAddress(),
		ip.PayloadLength(),
	)
	switch proto := ip.TransportProtocol(); proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(ip.Payload())
		tcp.SetChecksum(0)
		tcp.SetChecksum(^stdsum.Checksum(tcp, psum))
	case header.UDPProtocolNumber:
		udp := header.UDP(ip.Payload())
		udp.SetChecksum(0)
		udp.SetChecksum(^stdsum.Checksum(udp, psum))
	default:
		panic(fmt.Sprintf("not support protocol %d", proto))
	}

	if debug.Debug() {
		test.ValidIP(test.T(), ip)
	}
}
