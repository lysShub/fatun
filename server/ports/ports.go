package ports

import (
	"fmt"
	"itun/pack"
	"net/netip"
	"sync"
	"syscall"
)

type Ports struct {
	m *sync.RWMutex

	tcp map[uint16]fd
	udp map[uint16]fd

	locIP   netip.Addr
	version int
}

func NewPorts(laddr netip.Addr) (*Ports, error) {
	var r = &Ports{
		m:     &sync.RWMutex{},
		tcp:   map[uint16]fd{},
		udp:   map[uint16]fd{},
		locIP: laddr,
	}

	if laddr.Is4() || laddr.Is4In6() {
		r.version = 4
	} else if laddr.Is6() {
		r.version = 6
		panic("not support ipv6")
	} else {
		return nil, fmt.Errorf("invalid local address %s", laddr.String())
	}
	return r, nil
}

func (p *Ports) GetPort(proto pack.Proto) (uint16, error) {
	var (
		port uint16
		err  error
	)
	switch proto {
	case pack.TCP:
		port, err = p.bindTCP()
	case pack.UDP:
		port, err = p.bindUDP()
	default:
		panic("not support proto")
	}
	return port, err
}

func (p *Ports) ClosePort(proto pack.Proto, port uint16) error {
	p.m.Lock()
	defer p.m.Unlock()

	var h fd
	switch proto {
	case pack.TCP:
		h = p.tcp[port]
		delete(p.tcp, port)
	case pack.UDP:
		h = p.udp[port]
		delete(p.udp, port)
	default:
		return fmt.Errorf("not support proto %s", proto)
	}

	if h == 0 {
		return fmt.Errorf("not found bound port %d", port)
	} else {
		return syscall.Close(h)
	}
}

func (p *Ports) bindTCP() (uint16, error) {

	h, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		return 0, err
	}

	err = syscall.Bind(h, &syscall.SockaddrInet4{Addr: p.locIP.As4()})
	if err != nil {
		return 0, err
	}

	ls, err := syscall.Getsockname(fd(h))
	if err != nil {
		return 0, err
	}

	port := uint16(ls.(*syscall.SockaddrInet4).Port)
	p.m.Lock()
	p.tcp[port] = fd(h)
	p.m.Unlock()

	return port, nil
}

func (p *Ports) bindUDP() (uint16, error) {
	h, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return 0, err
	}

	err = syscall.Bind(h, &syscall.SockaddrInet4{Addr: p.locIP.As4()})
	if err != nil {
		return 0, err
	}

	ls, err := syscall.Getsockname(fd(h))
	if err != nil {
		return 0, err
	}

	port := uint16(ls.(*syscall.SockaddrInet4).Port)
	p.m.Lock()
	p.udp[port] = fd(h)
	p.m.Unlock()

	return port, nil
}

func (p *Ports) Close() (err error) {
	p.m.Lock()
	defer p.m.Unlock()

	for _, h := range p.tcp {
		if e := syscall.Close(h); e != nil && err == nil {
			err = e
		}
	}
	p.tcp = map[uint16]fd{}

	for _, h := range p.udp {
		if e := syscall.Close(h); e != nil && err == nil {
			err = e
		}
	}
	p.udp = map[uint16]fd{}

	return err
}
