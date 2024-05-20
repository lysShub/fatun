package ports

import (
	"net"
	"net/netip"
	"sync"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func NewMgr(addr netip.Addr) *Manager {
	if !addr.IsValid() {
		panic("invalid address")
	}

	var mgr = &Manager{
		tcp: map[uint16]*net.TCPListener{},
		udp: map[uint16]*net.UDPConn{},
	}
	mgr.tcpAddr = &net.TCPAddr{IP: addr.AsSlice()}
	mgr.udpAddr = &net.UDPAddr{IP: addr.AsSlice()}

	return mgr
}

type Manager struct {
	sync.RWMutex

	tcpAddr *net.TCPAddr
	tcp     map[uint16]*net.TCPListener

	udpAddr *net.UDPAddr
	udp     map[uint16]*net.UDPConn
}

func (p *Manager) GetTCPPort() (uint16, error) {
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

		if debug.Debug() {
			require.Greater(test.T(), addr.Port(), uint16(1024))
		}
		return addr.Port(), nil
	}
}

func (p *Manager) DelTCPPort(port uint16) error {
	p.Lock()
	l, ok := p.tcp[port]
	delete(p.tcp, port)
	p.Unlock()

	if !ok {
		return nil
	}
	return l.Close()
}

func (p *Manager) GetUDPPort() (uint16, error) {
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

		if debug.Debug() {
			require.Greater(test.T(), addr.Port(), uint16(1024))
		}
		return addr.Port(), nil
	}
}

func (p *Manager) DelUDPPort(port uint16) error {
	p.Lock()
	l, ok := p.udp[port]
	delete(p.udp, port)
	p.Unlock()

	if !ok {
		return nil
	}
	return l.Close()
}

func (p *Manager) Addr() netip.Addr {
	addr, ok := netip.AddrFromSlice(p.tcpAddr.IP)
	if !ok {
		panic(p.tcpAddr.IP)
	}
	return addr
}
