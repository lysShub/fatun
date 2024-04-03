//go:build windows
// +build windows

package capture

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/itun/errorx"
	sess "github.com/lysShub/itun/session"
	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/test"
	"github.com/lysShub/rsocket/test/debug"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type capture struct {
	opt *Option

	closed   atomic.Bool
	closeErr error

	hitter filter.Hitter

	captures   map[sess.Session]*session
	capturesMu sync.RWMutex

	sessCh chan *session

	srvCtx    context.Context
	srvCancel context.CancelFunc
}

var _ Capture = (*capture)(nil)

func newCapture(hit filter.Hitter, opt *Option) *capture {
	var c = &capture{
		opt: opt,

		hitter: hit,

		captures: make(map[sess.Session]*session, 16),
		sessCh:   make(chan *session, 16),
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	go c.tcpService()
	// go c.udpService()
	return c
}

func (s *capture) Close() error {
	s.close(os.ErrClosed)
	return nil
}

func (s *capture) close(cause error) {
	if s.closed.CompareAndSwap(false, true) {
		s.closeErr = cause

		s.srvCancel()

		close(s.sessCh)

		var cs []*session
		s.capturesMu.Lock()
		for _, e := range s.captures {
			cs = append(cs, e)
		}
		clear(s.captures)
		s.capturesMu.Unlock()

		for _, e := range cs {
			e.Close()
		}
	}
}

func (s *capture) Get(ctx context.Context) (Session, error) {
	select {
	case sess, ok := <-s.sessCh:
		if !ok {
			return nil, s.closeErr
		}
		return sess, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *capture) del(sess sess.Session) {
	s.capturesMu.Lock()
	defer s.capturesMu.Unlock()

	_, ok := s.captures[sess]
	if ok {
		delete(s.captures, sess)
	}
}

func (s *capture) tcpService() {
	filter := "outbound and !loopback and ip and tcp.Syn"
	d, err := divert.Open(filter, divert.Network, s.opt.Priority, 0)
	if err != nil {
		s.close(err)
		return
	}

	var addr divert.Address
	var p = make([]byte, s.opt.Mtu)
	for {
		n, err := d.RecvCtx(s.srvCtx, p, &addr)
		if err != nil {
			s.close(err)
			return
		}

		ip := header.IPv4(p[:n])
		tcp := header.TCP(ip.Payload())
		sess := sess.Session{
			Src:   netip.AddrPortFrom(toAddr(ip.SourceAddress()), tcp.SourcePort()),
			Proto: itun.TCP,
			Dst:   netip.AddrPortFrom(toAddr(ip.DestinationAddress()), tcp.DestinationPort()),
		}

		if !s.hitter.HitOnce(sess) {
			if _, err = d.Send(p[:n], outbound); err != nil {
				s.close(err)
				return
			}
		} else {
			s.capturesMu.RLock()
			_, ok := s.captures[sess] // todo: HitOnce 不需要这个map
			s.capturesMu.RUnlock()

			if !ok {
				c, err := newSession(s, sess, int(addr.Network().IfIdx), s.opt.Priority+1)
				if err != nil {
					s.opt.Logger.Warn(err.Error(), errorx.TraceAttr(err))
					continue
				}

				// only buff tcp first syn packet
				c.push(memcpy(p[:n]))

				select {
				case s.sessCh <- c:
					s.capturesMu.Lock()
					s.captures[sess] = c
					s.capturesMu.Unlock()
				default:
					c.Close()
					s.opt.Logger.Warn("xxx")
				}
			}
		}
	}
}

func (s *capture) udpService() {
	filter := "outbound and !loopback and ip and udp"
	d, err := divert.Open(filter, divert.Network, s.opt.Priority, 0)
	if err != nil {
		s.close(err)
		return
	}

	var addr divert.Address
	var p = make([]byte, s.opt.Mtu)
	for {
		n, err := d.RecvCtx(context.Background(), p, &addr)
		if err != nil {
			s.close(err)
			return
		}

		ip := header.IPv4(p[:n])
		udp := header.UDP(ip.Payload())
		sess := sess.Session{
			Src:   netip.AddrPortFrom(toAddr(ip.SourceAddress()), udp.SourcePort()),
			Proto: itun.UDP,
			Dst:   netip.AddrPortFrom(toAddr(ip.DestinationAddress()), udp.DestinationPort()),
		}

		if !s.hitter.HitOnce(sess) { // pass
			if _, err = d.Send(p[:n], outbound); err != nil {
				s.close(err)
				return
			}
		} else {
			s.capturesMu.RLock()
			c, ok := s.captures[sess]
			s.capturesMu.RUnlock()
			if !ok {
				c, err = newSession(s, sess, int(addr.Network().IfIdx), s.opt.Priority+1)
				if err != nil {
					s.opt.Logger.Warn(err.Error(), errorx.TraceAttr(err))
					continue
				}
			}

			c.push(memcpy(p[:n]))

			if !ok {
				select {
				case s.sessCh <- c:
					s.capturesMu.Lock()
					s.captures[sess] = c
					s.capturesMu.Lock()
				default:
					c.Close()
					s.opt.Logger.Warn("xxx")
				}
			}
		}
	}
}

func memcpy(src []byte) []byte {
	tmp := make([]byte, len(src))
	copy(tmp, src)
	return tmp
}

var outbound = &divert.Address{}

func init() {
	outbound.SetOutbound(true)
}

func toAddr(a tcpip.Address) netip.Addr {
	if a.Len() == 4 {
		return netip.AddrFrom4(a.As4())
	} else {
		return netip.AddrFrom16(a.As16())
	}
}

type session struct {
	capture *capture
	s       sess.Session

	closed atomic.Bool

	// before session work, some packet maybe read by capture
	prevIPsCh chan []byte

	d *divert.Handle

	inboundAddr *divert.Address
	ipstack     *rsocket.IPStack
}

var _ Session = (*session)(nil)

func newSession(
	capture *capture, s sess.Session,
	injectIfIdx int, priority int16,
) (*session, error) {
	var err error
	var c = &session{
		capture: capture,
		s:       s,

		prevIPsCh: make(chan []byte, 4),

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
	c.ipstack, err = rsocket.NewIPStack(s.Src.Addr(), s.Dst.Addr(), tcpip.TransportProtocolNumber(s.Proto))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *session) push(ip []byte) {
	select {
	case c.prevIPsCh <- ip:
	default:
	}
}

func (c *session) Capture(ctx context.Context, pkt *rsocket.Packet) (err error) {
	b := pkt.Data()
	b = b[:cap(b)]

	select {
	case ip := <-c.prevIPsCh:
		hdrn := uint8(0)
		switch header.IPVersion(ip) {
		case 4:
			hdrn = header.IPv4(ip).HeaderLength()
		case 6:
			hdrn = header.IPv6MinimumSize
		default:
		}

		n := copy(b, ip[hdrn:])
		pkt.SetLen(n)
		return nil
	default:
	}

	n, err := c.d.RecvCtx(ctx, b, nil)
	if err != nil {
		pkt.SetLen(0)
		return err
	}
	pkt.SetLen(n)

	// todo: ipv6
	iphdrLen := header.IPv4(pkt.Data()).HeaderLength()
	pkt.SetHead(pkt.Head() + int(iphdrLen))

	return nil
}

func (c *session) Inject(pkt *rsocket.Packet) error {
	c.ipstack.AttachInbound(pkt)
	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Data())
	}

	_, err := c.d.Send(pkt.Data(), c.inboundAddr)
	return err
}

func (c *session) Session() sess.Session { return c.s }
func (s *session) String() string        { return s.s.String() }

func (c *session) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.capture.del(c.s)
		close(c.prevIPsCh)
		return errors.WithStack(c.d.Close())
	}
	return nil
}
