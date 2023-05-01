package ports

import (
	"errors"
	"itun/pack"
	"net"
	"net/netip"
	"sync"
	"syscall"
)

// 五元组确定一个连接(这里可以忽略locIP)
// 但是这里有两个连接
/*
	m1:	map[{proxyId+srcPort+proto}]locPort

	m2:	map[locPort][]{proto+dstAddr}

		client来了一个数据报, 通过{proxyId+srcPort+proto}查表,
	如果表中存在, 这直接使用此locPort发送。

		如果表中不存在, 则表明是新session, 尝试获取新locPort: 遍历
	m2, 如果val中没用相同的值则使用此locPort, 否则获取新的locPort


	将m1移动到proxyer中去
*/

type PortMgr interface {
	GetPort(proto pack.Proto, dstAddr netip.AddrPort) (locPort uint16, err error)
	DelPort(proto pack.Proto, dstAddr netip.AddrPort, locPort uint16)
}

/*

  端口映射管理, 基于5原组确定一个连接:
	一个AddrPort对应多个locPort
	一个locPort对应多个AddrPort

  为了减少本地端口的消耗, 如果一个LocPort对应的DstAddrs中没有当前的DstAddr,
  那么这个端口就可用于向DstAddr发送数据.

  同时, 为了避免和本地网络冲突, 通过只Bind的方法占用系统端口。

*/

type ports struct {
	locIP netip.Addr

	udps []*port
	tcps []*port

	m *sync.RWMutex

	_locSockaddr syscall.Sockaddr
}

var _ PortMgr = &ports{}

func NewPorts(locIP netip.Addr) *ports {
	return &ports{
		locIP: locIP,
		m:     &sync.RWMutex{},
	}
}

func (p *ports) GetPort(proto pack.Proto, dstAddr netip.AddrPort) (locPort uint16, err error) {
	switch proto {
	case pack.UDP:
		return p.getUDPPort(dstAddr)
	case pack.TCP:
		return p.getTCPPort(dstAddr)
	default:
		panic("")
	}
}

func (p *ports) DelPort(proto pack.Proto, dstAddr netip.AddrPort, locPort uint16) {
	p.m.RLock()
	defer p.m.RUnlock()

	switch proto {
	case pack.UDP:
		for _, v := range p.udps {
			if v.Del(locPort, dstAddr) {
				return
			}
		}
	case pack.TCP:
		for _, v := range p.tcps {
			if v.Del(locPort, dstAddr) {
				return
			}
		}
	default:
		panic("")
	}
}

func (p *ports) getUDPPort(dstAddr netip.AddrPort) (uint16, error) {
	p.m.RLock()
	for _, v := range p.udps {
		if v.Put(dstAddr) {
			p.m.RUnlock()
			return v.LocPort(), nil
		}
	}
	p.m.RUnlock()

	// new port
	port, fd, err := p.bind(pack.UDP)
	if err != nil {
		return 0, err
	}
	np := newPort(port, fd)
	np.Put(dstAddr)

	p.m.Lock()
	p.udps = append(p.udps, np)
	p.m.Unlock()
	return port, nil
}

func (p *ports) getTCPPort(dstAddr netip.AddrPort) (uint16, error) {
	p.m.RLock()
	for _, v := range p.tcps {
		if v.Put(dstAddr) {
			p.m.RUnlock()
			return v.LocPort(), nil
		}
	}
	p.m.RUnlock()

	// new port
	port, fd, err := p.bind(pack.TCP)
	if err != nil {
		return 0, err
	}
	np := newPort(port, fd)
	np.Put(dstAddr)

	p.m.Lock()
	p.tcps = append(p.tcps, np)
	p.m.Unlock()
	return port, nil
}

func (p *ports) bind(proto pack.Proto) (uint16, int, error) {
	switch proto {
	case pack.UDP:
		return p.bindUDP()
	case pack.TCP:
		return p.bindTCP()
	default:
		panic("")
	}
}

func (p *ports) locSockaddr() syscall.Sockaddr {
	p.m.RLock()
	if p._locSockaddr != nil {
		p.m.RUnlock()
		return p._locSockaddr
	}
	p.m.Unlock()

	p.m.Lock()
	if p.locIP.Is6() {
		p._locSockaddr = &syscall.SockaddrInet6{ZoneId: getZoneIdx(p.locIP), Addr: p.locIP.As16()}
	} else {
		p._locSockaddr = &syscall.SockaddrInet4{Addr: p.locIP.As4()}
	}
	p.m.Unlock()

	return p.locSockaddr()
}

func (p *ports) bindUDP() (uint16, int, error) {
	var laddr = p.locSockaddr()

	switch a := laddr.(type) {
	case *syscall.SockaddrInet4:
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		if err != nil {
			return 0, 0, err
		}
		if err = syscall.Bind(fd, laddr); err != nil {
			return 0, 0, err
		}
		return uint16(a.Port), int(fd), nil
	case *syscall.SockaddrInet6:
		fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		if err != nil {
			return 0, 0, err
		}
		if err = syscall.Bind(fd, laddr); err != nil {
			return 0, 0, err
		}
		return uint16(a.Port), int(fd), nil
	default:
		panic("")
	}
}

func (p *ports) bindTCP() (uint16, int, error) {
	var laddr = p.locSockaddr()

	switch a := laddr.(type) {
	case *syscall.SockaddrInet4:
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
		if err != nil {
			return 0, 0, err
		}
		if err = syscall.Bind(fd, laddr); err != nil {
			return 0, 0, err
		}
		return uint16(a.Port), int(fd), nil
	case *syscall.SockaddrInet6:
		fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
		if err != nil {
			return 0, 0, err
		}
		if err = syscall.Bind(fd, laddr); err != nil {
			return 0, 0, err
		}
		return uint16(a.Port), int(fd), nil
	default:
		panic("")
	}
}

func getZoneIdx(locIP netip.Addr) uint32 {
	ifis, _ := net.Interfaces()
	for _, ifi := range ifis {
		addrs, _ := ifi.Addrs()
		for _, addr := range addrs {
			a, _ := netip.ParseAddr(addr.String())
			if a == locIP {
				return uint32(ifi.Index)
			}
		}
	}
	return 0
}

/*


 */

type port struct {
	port     uint16
	fd       int
	dstAddrs map[netip.AddrPort]struct{}

	m *sync.RWMutex
}

func newPort(locPort uint16, fd int) *port {
	return &port{
		port:     locPort,
		fd:       fd,
		dstAddrs: map[netip.AddrPort]struct{}{},
		m:        &sync.RWMutex{},
	}
}

func (e *port) Put(dst netip.AddrPort) (ok bool) {
	if e.has(dst) {
		return false
	}

	e.m.Lock()
	e.dstAddrs[dst] = struct{}{}
	e.m.Unlock()
	return true
}

func (e *port) Del(locPort uint16, dst netip.AddrPort) (ok bool) {
	if e.port != locPort || !e.has(dst) {
		return false
	}

	e.m.Lock()
	delete(e.dstAddrs, dst)
	e.m.Unlock()
	return true
}

func (e *port) Len() (n int) {
	e.m.RLock()
	n = len(e.dstAddrs)
	e.m.RUnlock()
	return n
}

func (e *port) Close() error {
	e.m.Lock()
	defer e.m.Unlock()

	if len(e.dstAddrs) > 0 {
		return errors.New("can not close not null port")
	}
	if err := syscall.Close(syscall.Handle(e.fd)); err != nil {
		return err
	}
	e.port, e.fd = 0, 0
	return nil
}

func (e *port) LocPort() uint16 {
	return e.port
}

func (p *port) has(dst netip.AddrPort) bool {
	p.m.RLock()
	defer p.m.RUnlock()

	var has = false
	if len(p.dstAddrs) == 0 {
		has = false
	} else {
		_, has = p.dstAddrs[dst]
	}
	return has
}
