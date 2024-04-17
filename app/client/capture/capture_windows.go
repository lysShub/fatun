//go:build windows
// +build windows

package capture

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/client/filter"
	sess "github.com/lysShub/itun/session"
	"github.com/lysShub/sockit/helper/ipstack"
	"github.com/lysShub/sockit/packet"

	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type capture struct {
	opt *Config

	handle *divert.Handle
	addr   divert.Address

	hitter filter.Hitter

	closeErr atomic.Pointer[error]
}

var _ Capture = (*capture)(nil)

func newCapture(hit filter.Hitter, opt *Config) (*capture, error) {
	var c = &capture{
		opt:    opt,
		hitter: hit,
	}

	// for performance, only capture tcp.Syn packet
	// todo: support icmp
	var filter = "outbound and !loopback and ip and (tcp.Syn or udp)"

	var err error
	c.handle, err = divert.Open(filter, divert.Network, opt.Priority, 0)
	if err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func (s *capture) close(cause error) error {
	if s.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if s.handle != nil {
			if err := s.handle.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			s.closeErr.Store(&cause)
		}
		return cause
	}
	return *s.closeErr.Load()
}

func (s *capture) Capture(ctx context.Context) (Session, error) {
	var ip = make([]byte, s.opt.Mtu)
	for {
		n, err := s.handle.RecvCtx(ctx, ip[:cap(ip)], &s.addr)
		if err != nil {
			return nil, s.close(err)
		}
		ip = ip[:n]

		if getsession(ip).Src.Port() == 12345 {
			print()
		}

		if hit, err := s.hitter.Hit(ip); err != nil {
			if !errors.Is(err, filter.ErrNotRecord{}) {
				return nil, s.close(err)
			}
			// s.opt.Logger.LogAttrs(ctx, slog.LevelWarn, err.Error(), slog.String("session", getsession(ip).String()))
			// s.opt.Logger.Warn(err.Error(), errorx.TraceAttr(err))
		} else {
			if s := getsession(ip); s.Proto == itun.UDP {
				fmt.Println("有有有有有有有有有有有有有有有有有有有有", s.String())
			}

			if !hit {
				if _, err = s.handle.Send(ip[:n], &s.addr); err != nil {
					return nil, s.close(err)
				}
			} else {
				sess := getsession(ip)
				if sess.IsValid() {
					return nil, errors.Errorf("capture invalid ip packet: %s", hex.EncodeToString(ip))
				}

				c, err := newSession(sess, ip, int(s.addr.Network().IfIdx), s.opt.Priority+1)
				return c, err
			}
		}
	}
}

func (s *capture) Close() error { return s.close(os.ErrClosed) }

func getsession(ip []byte) sess.Session {
	var (
		s     sess.Session
		iphdr header.Network
		hdr   header.Transport
	)
	switch header.IPVersion(ip) {
	case 4:
		iphdr = header.IPv4(ip)
		s.Src = netip.AddrPortFrom(netip.AddrFrom4(iphdr.SourceAddress().As4()), 0)
		s.Dst = netip.AddrPortFrom(netip.AddrFrom4(iphdr.DestinationAddress().As4()), 0)
	case 6:
		iphdr = header.IPv6(ip)
		s.Src = netip.AddrPortFrom(netip.AddrFrom16(iphdr.SourceAddress().As16()), 0)
		s.Dst = netip.AddrPortFrom(netip.AddrFrom16(iphdr.DestinationAddress().As16()), 0)
	default:
		return sess.Session{}
	}
	switch iphdr.TransportProtocol() {
	case header.TCPProtocolNumber:
		s.Proto = itun.TCP
		hdr = header.TCP(iphdr.Payload())
	case header.UDPProtocolNumber:
		s.Proto = itun.UDP
		hdr = header.UDP(iphdr.Payload())
	default:
		return sess.Session{}
	}
	s.Src = netip.AddrPortFrom(s.Src.Addr(), hdr.SourcePort())
	s.Dst = netip.AddrPortFrom(s.Dst.Addr(), hdr.SourcePort())
	return s
}

type session struct {
	s sess.Session
	d *divert.Handle

	initPack    []byte
	inboundAddr *divert.Address
	ipstack     *ipstack.IPStack

	closeErr atomic.Pointer[error]
}

var _ Session = (*session)(nil)

func newSession(
	s sess.Session, initPacket []byte,
	injectIfIdx int, priority int16,
) (*session, error) {
	var err error
	var c = &session{
		s: s,

		initPack:    initPacket,
		inboundAddr: &divert.Address{},
	}
	c.inboundAddr.Network().IfIdx = uint32(injectIfIdx)

	filter := fmt.Sprintf(
		"%s and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d",
		strings.ToLower(s.Proto.String()), s.Src.Addr(), s.Src.Port(), s.Dst.Addr(), s.Dst.Port(),
	)

	c.d, err = divert.Open(filter, divert.Network, priority, 0)
	if err != nil {
		return nil, err
	}
	c.ipstack, err = ipstack.New(s.Src.Addr(), s.Dst.Addr(), tcpip.TransportProtocolNumber(s.Proto))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *session) Capture(ctx context.Context, pkt *packet.Packet) (err error) {
	if len(c.initPack) > 0 {
		pkt.SetData(0).Append(c.initPack)
		c.initPack = nil
		return nil
	}

	b := pkt.Bytes()
	n, err := c.d.RecvCtx(ctx, b[:cap(b)], nil)
	if err != nil {
		pkt.SetData(0)
		return err
	}
	pkt.SetData(n)

	// todo: ipv6
	iphdrLen := header.IPv4(pkt.Bytes()).HeaderLength()
	pkt.SetHead(pkt.Head() + int(iphdrLen))
	return nil
}

func (c *session) Inject(pkt *packet.Packet) error {
	c.ipstack.AttachInbound(pkt)
	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Bytes())
	}

	_, err := c.d.Send(pkt.Bytes(), c.inboundAddr)
	return err
}

func (c *session) Session() sess.Session { return c.s }
func (s *session) String() string        { return s.s.String() }

func (c *session) Close() error { return c.close(nil) }

func (c *session) close(cause error) error {
	if c.closeErr.CompareAndSwap(nil, &net.ErrClosed) {
		if c.d != nil {
			if err := c.d.Close(); cause != nil {
				cause = err
			}
		}

		if cause != nil {
			c.closeErr.Store(&cause)
		}
		return cause
	}
	return *c.closeErr.Load()
}
