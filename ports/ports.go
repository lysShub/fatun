package ports

import (
	"net"
	"net/netip"
	"sync"
)

func NewPortMgr(addr netip.Addr) *PortMgr {
	if !addr.IsValid() {
		panic("invalid address")
	}

	var mgr = &PortMgr{
		tcp: map[uint16]*net.TCPListener{},
		udp: map[uint16]*net.UDPConn{},
	}
	mgr.tcpAddr = &net.TCPAddr{IP: addr.AsSlice()}
	mgr.udpAddr = &net.UDPAddr{IP: addr.AsSlice()}

	return mgr
}

type PortMgr struct {
	sync.RWMutex

	tcpAddr *net.TCPAddr
	tcp     map[uint16]*net.TCPListener

	udpAddr *net.UDPAddr
	udp     map[uint16]*net.UDPConn
}

func (p *PortMgr) GetTCPPort() (uint16, error) {
	l, err := net.ListenTCP("tcp", p.tcpAddr)
	if err != nil {
		return 0, err
	}

	if err := setFilterAll(l); err != nil {
		return 0, err
	}

	addr, err := netip.ParseAddrPort(l.Addr().String())
	if err != nil {
		return 0, err
	} else {
		p.Lock()
		p.tcp[addr.Port()] = l
		p.Unlock()

		return addr.Port(), nil
	}
}

func (p *PortMgr) DelTCPPort(port uint16) error {
	p.Lock()
	l, ok := p.tcp[port]
	delete(p.tcp, port)
	p.Unlock()

	if !ok {
		return nil
	}
	return l.Close()
}

func (p *PortMgr) GetUDPPort() (uint16, error) {
	c, err := net.ListenUDP("udp", p.udpAddr)
	if err != nil {
		return 0, err
	}

	if err := setFilterAll(c); err != nil {
		return 0, err
	}

	addr, err := netip.ParseAddrPort(c.LocalAddr().String())
	if err != nil {
		return 0, err
	} else {
		p.Lock()
		p.udp[addr.Port()] = c
		p.Unlock()

		return addr.Port(), nil
	}
}

func (p *PortMgr) DelUDPPort(port uint16) error {
	p.Lock()
	l, ok := p.udp[port]
	delete(p.udp, port)
	p.Unlock()

	if !ok {
		return nil
	}
	return l.Close()
}
