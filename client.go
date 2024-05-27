package fatun

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/netip"
	"os"

	"github.com/lysShub/fatun/checksum"
	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	stdsum "gvisor.dev/gvisor/pkg/tcpip/checksum"
)

type Capturer interface {
	Capture(ip *packet.Packet) error
	Inject(ip *packet.Packet) error
	Close() error
}

type Client struct {
	// Logger Warn/Error logger
	Logger *slog.Logger

	Conn conn.Conn

	Capturer Capturer

	peer     conn.Peer
	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr errorx.CloseErr
}

func NewClient[P conn.Peer](opts ...func(*Client)) (*Client, error) {
	var c = &Client{peer: *new(P)}
	c.srvCtx, c.cancel = context.WithCancel(context.Background())

	for _, opt := range opts {
		opt(c)
	}

	if c.Logger == nil {
		c.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	if c.Conn == nil {
		return nil, errors.New("require conn.Conn")
	}

	var err error
	if c.Capturer == nil {
		c.Capturer, err = NewDefaultCapture(c.Conn.LocalAddr(), c.Conn.Overhead())
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Client) Run() {
	go c.uplinkService()
	go c.downlinkServic()
}

func (c *Client) close(cause error) (_ error) {
	if cause != nil {
		c.Logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		c.Logger.Info("client close", errorx.Trace(nil))
	}

	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)

		if c.cancel != nil {
			c.cancel()
		}
		if c.Capturer != nil {
			errs = append(errs, c.Capturer.Close())
		}
		if c.Conn != nil {
			errs = append(errs, c.Conn.Close())
		}
		return
	})
}

func (c *Client) uplinkService() (_ error) {
	var (
		ip   = packet.Make(64, c.Conn.MTU())
		peer = c.peer.Builtin().Reset(0, netip.IPv4Unspecified())
	)

	for {
		err := c.Capturer.Capture(ip.Sets(64, 0xffff))
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
		if err = c.Conn.Send(peer, pkt); err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) downlinkServic() error {
	var (
		pkt  = packet.Make(0, c.Conn.MTU())
		peer = c.peer.Builtin().Reset(0, netip.IPv4Unspecified())
	)

	for {
		err := c.Conn.Recv(peer, pkt.Sets(0, 0xffff))
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

		if err = c.Capturer.Inject(pkt); err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) Close() error { return c.close(nil) }

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

func UpdateTcpMssOption(hdr header.TCP, delta int) error {
	n := int(hdr.DataOffset())
	if n > header.TCPMinimumSize && delta != 0 {
		oldSum := ^hdr.Checksum()
		for i := header.TCPMinimumSize; i < n; {
			kind := hdr[i]
			switch kind {
			case header.TCPOptionMSS:
				/* {kind} {length} {max seg size} */
				if i+4 <= n && hdr[i+1] == 4 {
					old := binary.BigEndian.Uint16(hdr[i+2:])
					new := int(old) + delta
					if new <= 0 {
						return errors.Errorf("updated mss is invalid %d", new)
					}

					if (i+2)%2 == 0 {
						binary.BigEndian.PutUint16(hdr[i+2:], uint16(new))
						sum := stdsum.Combine(stdsum.Combine(oldSum, ^old), uint16(new))
						hdr.SetChecksum(^sum)
					} else if i+5 <= n {
						sum := stdsum.Combine(oldSum, ^stdsum.Checksum(hdr[i+1:i+5], 0))

						binary.BigEndian.PutUint16(hdr[i+2:], uint16(new))

						sum = stdsum.Combine(sum, stdsum.Checksum(hdr[i+1:i+5], 0))
						hdr.SetChecksum(^sum)
					}
					return nil
				} else {
					return errors.Errorf("invalid tcp packet: %s", hex.EncodeToString(hdr[:n]))
				}
			case header.TCPOptionNOP:
				i += 1
			case header.TCPOptionEOL:
				return nil // not mss opt
			default:
				if i+1 < n {
					i += int(hdr[i+1])
				} else {
					return errors.Errorf("invalid tcp packet: %s", hex.EncodeToString(hdr[:n]))
				}
			}
		}
	}
	return nil
}
