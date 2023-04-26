package user

import (
	"net"
	"net/netip"
	"sync"

	"itun/server/bpf"

	"github.com/mdlayher/packet"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

type capture struct {
	*packet.Conn

	m map[bpf.Pcap]struct{}
	l *sync.RWMutex
}

func NewCapture(ifi *net.Interface) (*capture, error) {
	var c = &capture{
		m: map[bpf.Pcap]struct{}{},
		l: &sync.RWMutex{},
	}
	var err error
	c.Conn, err = packet.Listen(ifi, packet.Raw, unix.ETH_P_ALL, &packet.Config{Filter: bpf.None})
	if err != nil {
		return nil, err
	}

	return c, err
}

func (c *capture) Add(filter bpf.Pcap) error {
	has := false
	c.l.RLock()
	_, has = c.m[filter]
	c.l.RUnlock()
	if has {
		return nil
	} else {
		c.l.Lock()
		c.m[filter] = struct{}{}
		c.l.RLock()
	}

	var fs bpf.Filter
	for f := range c.m {
		fs = append(fs, f)
	}
	raw, err := fs.Assemble()
	if err != nil {
		return err
	}
	return c.Conn.SetBPF(raw)
}

func (c *capture) Del(filter bpf.Pcap) error {
	has := false
	c.l.RLock()
	_, has = c.m[filter]
	c.l.RUnlock()
	if !has {
		return nil
	} else {
		c.l.Lock()
		delete(c.m, filter)
		c.l.RLock()
	}

	var fs bpf.Filter
	for f := range c.m {
		fs = append(fs, f)
	}
	raw, err := fs.Assemble()
	if err != nil {
		return err
	}
	return c.Conn.SetBPF(raw)
}

func (c *capture) ReadFrom(b []byte) (n int, proto uint8, addr netip.Addr, err error) {
	n, _, err = c.Conn.ReadFrom(b)
	if err != nil {
		return
	}

	const ethHdrLen = 14
	h, err := ipv4.ParseHeader(b[ethHdrLen:n])
	if err != nil {
		return
	}
	addr = netip.AddrFrom4([4]byte(h.Src))
	return
}

func (c *capture) WriteTo(b []byte) error {
	return nil
}
