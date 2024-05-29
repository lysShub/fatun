package audp

import (
	"net"
	"net/netip"
	"sync"

	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type acceptConn struct {
	l     *Listener
	raddr netip.AddrPort

	mu         sync.RWMutex
	putNotify  *sync.Cond
	segs       []segment
	sart, size int
	closed     bool
}

var _ Conn = (*acceptConn)(nil)

func newAcceptConn(l *Listener, raddr netip.AddrPort) *acceptConn {
	var c = &acceptConn{l: l, raddr: raddr}
	c.putNotify = sync.NewCond(&c.mu)

	c.segs = make([]segment, 128) // todo: from config
	return c
}

func (c *acceptConn) Write(b []byte) (int, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, errors.WithStack(net.ErrClosed)
	}
	c.mu.RUnlock()
	return c.l.udp.WriteToUDPAddrPort(b, c.raddr)
}

func (c *acceptConn) Read(b []byte) (int, error) {
	seg, err := c.pop()
	if err != nil {
		return 0, err
	}
	defer func() { c.l.put(seg) }()

	n := copy(b, *seg)
	if n != len(*seg) {
		return n, errorx.ShortBuff(len(*seg), n)
	}
	return n, nil
}

func (c *acceptConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: c.l.Addr().Addr().AsSlice(), Port: int(c.l.Addr().Port())}
}
func (c *acceptConn) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: c.raddr.Addr().AsSlice(), Port: int(c.raddr.Port())}
}
func (c *acceptConn) destroy() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}
func (c *acceptConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true

	c.l.del(c.raddr)
	for i := 0; i < c.size; i++ {
		c.l.put(c.popLocked())
	}
	return nil
}

type segment = *segmentData
type segmentData []byte

func (s *segmentData) full() {
	*s = (*s)[:cap(*s)]
}
func (s *segmentData) data(n int) {
	*s = (*s)[:n]
}

func (c *acceptConn) pop() (segment, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, errors.WithStack(net.ErrClosed)
	}
	return c.popLocked(), nil
}
func (c *acceptConn) popLocked() segment {
	for c.size == 0 {
		c.putNotify.Wait()
	}

	val := c.segs[c.sart]

	c.size -= 1
	c.sart = (c.sart + 1)
	if c.sart >= len(c.segs) {
		c.sart = c.sart - len(c.segs)
	}
	return val
}
func (c *acceptConn) put(t segment) error {
	if t == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.WithStack(net.ErrClosed)
	}
	c.putLocked(t)
	return nil
}
func (c *acceptConn) putLocked(t segment) {
	if len(c.segs) == c.size {
		c.l.put(c.popLocked())
	}

	i := c.sart + c.size
	if i >= len(c.segs) {
		i = i - len(c.segs)
	}

	c.segs[i] = t
	c.size += 1
	c.putNotify.Signal()
}
