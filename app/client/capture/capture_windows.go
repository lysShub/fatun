//go:build windows
// +build windows

package capture

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	pkge "github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/itun/cctx"
	sess "github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Option struct {
	// NIC  int
	// IPv6 bool

	Prots []itun.Proto // default tcp

	Logger slog.Handler
}

type capture struct {
	ctx      cctx.CancelCtx
	logger   *slog.Logger
	mtu      int
	closed   atomic.Bool
	priority int16

	hitter filter.Hitter

	// hited connect captures packet, will captures
	// connect serval start packet
	captures   map[sess.Session]*session
	capturesMu sync.RWMutex

	ch chan *session
}

var _ Capture = (*capture)(nil)

func newCapture(ctx cctx.CancelCtx, hit filter.Hitter, opt *Option) *capture {
	// todo:
	opt = &Option{
		Prots:  []itun.Proto{itun.TCP},
		Logger: slog.NewJSONHandler(os.Stderr, nil),
	}

	var c = &capture{
		ctx:      ctx,
		hitter:   hit,
		logger:   slog.New(opt.Logger).WithGroup("capture"),
		mtu:      1536, // todo
		priority: 99,

		captures: make(map[sess.Session]*session, 16),
		ch:       make(chan *session, 16),
	}

	go c.tcpService()
	// go c.udpService()
	return c
}

func (s *capture) Close(cause error) error {
	if s.closed.CompareAndSwap(false, true) {
		s.ctx.Cancel(cause)
		close(s.ch)

		var cs []*session
		s.capturesMu.RLock()
		for _, e := range s.captures {
			cs = append(cs, e)
		}
		s.capturesMu.RUnlock()

		for _, e := range cs {
			s.Del(e.Session())
		}
	}
	return nil
}

func (s *capture) Get(ctx context.Context) (Session, error) {
	select {
	case sess, ok := <-s.ch:
		if !ok {
			return nil, pkge.WithStack(s.ctx.Err())
		}
		return sess, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *capture) Del(sess sess.Session) {
	s.capturesMu.Lock()
	c, ok := s.captures[sess]
	if ok {
		delete(s.captures, sess)
	}
	s.capturesMu.Unlock()

	if ok {
		c.close()
	}
}

func (s *capture) tcpService() error {
	filter := "outbound and !loopback and ip and tcp.Syn"
	d, err := divert.Open(filter, divert.NETWORK, s.priority, 0)
	if err != nil {
		return s.Close(err)
	}

	var addr divert.Address
	var p = make([]byte, s.mtu)
	for {
		n, err := d.RecvCtx(s.ctx, p, &addr)
		if err != nil {
			return s.Close(err)
		}

		ip := header.IPv4(p[:n])
		udp := header.UDP(ip.Payload())
		sess := sess.Session{
			Src:   netip.AddrPortFrom(toAddr(ip.SourceAddress()), udp.SourcePort()),
			Proto: itun.TCP,
			Dst:   netip.AddrPortFrom(toAddr(ip.DestinationAddress()), udp.DestinationPort()),
		}

		if !s.hitter.HitOnce(sess) { // pass
			if _, err = d.Send(p[:n], outbound); err != nil {
				return s.Close(err)
			}
		} else {
			s.capturesMu.RLock()
			_, ok := s.captures[sess] // todo: HitOnce 不需要这个map
			s.capturesMu.RUnlock()

			if !ok {
				c, err := newSession(sess, int(addr.Network().IfIdx), s.priority+1)
				if err != nil {
					s.logger.Warn(err.Error(), app.TraceAttr(err))
					continue
				}

				s.capturesMu.Lock()
				s.captures[sess] = c
				s.capturesMu.Unlock()

				// only buff tcp first syn packet
				c.push(memcpy(p[:n]))

				select {
				case s.ch <- c:
				default:
					s.Del(sess)
					s.logger.Warn("xxx")
				}
			}
		}
	}
}

func (s *capture) udpService() error {
	filter := "outbound and !loopback and ip and udp"
	d, err := divert.Open(filter, divert.NETWORK, s.priority, 0)
	if err != nil {
		return s.Close(err)
	}

	var addr divert.Address
	var p = make([]byte, s.mtu)
	for {
		n, err := d.RecvCtx(context.Background(), p, &addr)
		if err != nil {
			return s.Close(err)
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
				return s.Close(err)
			}
		} else {
			s.capturesMu.RLock()
			c, ok := s.captures[sess]
			s.capturesMu.RUnlock()

			if !ok {
				c, err = newSession(sess, int(addr.Network().IfIdx), s.priority+1)
				if err != nil {
					s.logger.Warn(err.Error(), app.TraceAttr(err))
					continue
				}

				s.capturesMu.Lock()
				s.captures[sess] = c
				s.capturesMu.Lock()
			}

			c.push(memcpy(p[:n]))

			select {
			case s.ch <- c:
			default:
				s.Del(sess)
				s.logger.Warn("xxx")
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
	s sess.Session

	buff chan []byte
	d    *divert.Divert

	inboundAddr *divert.Address
	ipstack     *relraw.IPStack

	closed atomic.Bool
}

func newSession(
	s sess.Session, injectIfIdx int, priority int16,
) (*session, error) {
	var err error
	var c = &session{
		s:           s,
		inboundAddr: &divert.Address{},
	}
	c.inboundAddr.Network().IfIdx = uint32(injectIfIdx)

	filter := fmt.Sprintf(
		"%s and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d",
		strings.ToLower(s.Proto.String()), s.Src.Addr(), s.Src.Port(), s.Dst.Addr(), s.Dst.Port(),
	)

	c.d, err = divert.Open(filter, divert.NETWORK, priority, 0)
	if err != nil {
		return nil, err
	}
	c.ipstack, err = relraw.NewIPStack(s.Src.Addr(), s.Dst.Addr(), tcpip.TransportProtocolNumber(s.Proto))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *session) push(b []byte) {
	select {
	case c.buff <- b:
	default:
	}
}

var _ Session = (*session)(nil)

func (c *session) Capture(ctx context.Context, pkt *relraw.Packet) (err error) {
	b := pkt.Data()
	b = b[:cap(b)]

	select {
	case tmp := <-c.buff:
		n := copy(b, tmp)
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

	if debug.Debug() {
		require.Equal(test.T(), header.IPVersion(pkt.Data()), 4)
	}
	iphdrLen := header.IPv4(pkt.Data()).HeaderLength()
	pkt.SetHead(pkt.Head() + int(iphdrLen))

	return nil
}

func (c *session) Inject(pkt *relraw.Packet) error {
	c.ipstack.AttachInbound(pkt)
	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Data())
	}

	_, err := c.d.Send(pkt.Data(), c.inboundAddr)
	return err
}

func (c *session) Session() sess.Session {
	return c.s
}

func (c *session) close() error {
	if c.closed.CompareAndSwap(false, true) {
		return c.d.Close()
	}
	return nil
}

func (s *session) String() string { return s.s.String() }
