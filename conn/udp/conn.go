package udp

import (
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

func Dial(laddr, raddr *net.UDPAddr) (*net.UDPConn, error) {
	return net.DialUDP("udp", laddr, raddr)
}

type acceptConn struct {
	l     *Listener
	raddr netip.AddrPort

	closed atomic.Bool
	buff   chan segment
}

var _ net.Conn = (*acceptConn)(nil)

func newAcceptConn(l *Listener, raddr netip.AddrPort) *acceptConn {
	var c = &acceptConn{
		l: l, raddr: raddr,
		buff: make(chan *segmentData, 128), // todo: from config
	}
	return c
}

func (c *acceptConn) Write(b []byte) (int, error) {
	if c.closed.Load() {
		return 0, errors.WithStack(net.ErrClosed)
	}
	return c.l.udp.WriteToUDPAddrPort(b, c.raddr)
}

func (c *acceptConn) Read(b []byte) (int, error) {
	seg, ok := <-c.buff
	if !ok {
		return 0, errors.WithStack(net.ErrClosed)
	}
	defer func() { c.l.put(seg) }()

	n := copy(b, *seg)
	if n != len(*seg) {
		return n, errorx.ShortBuff(len(*seg), n)
	}
	return n, nil
}

func (c *acceptConn) LocalAddr() net.Addr { return c.l.Addr() }
func (c *acceptConn) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: c.raddr.Addr().AsSlice(), Port: int(c.raddr.Port())}
}
func (c *acceptConn) SetDeadline(t time.Time) error      { panic("todo: ref gvisor gonet") }
func (c *acceptConn) SetWriteDeadline(t time.Time) error { panic("todo: ref gvisor gonet") }
func (c *acceptConn) SetReadDeadline(t time.Time) error  { panic("todo: ref gvisor gonet") }

func (c *acceptConn) Close() error {
	c.closed.Store(true)
	close(c.buff)

	c.l.del(c.raddr)
	for e := range c.buff {
		c.l.put(e)
	}
	return nil
}

func (c *acceptConn) put(s segment) {
	for !c.closed.Load() {
		select {
		case c.buff <- s: // probably painc write closed ch
			return
		default:
			c.l.put(<-c.buff)
		}
	}
}

type segment = *segmentData
type segmentData []byte

func (s *segmentData) full() {
	*s = (*s)[:cap(*s)]
}
func (s *segmentData) data(n int) {
	*s = (*s)[:n]
}
