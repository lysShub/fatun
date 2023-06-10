package maps

import (
	"fmt"
	"itun/pack"
	"net/netip"
	"sync"
)

type downMap struct {
	tcp  map[de]netip.AddrPort
	tcpm *sync.RWMutex

	udp  map[de]netip.AddrPort
	udpm *sync.RWMutex
}

type de struct {
	dst     netip.AddrPort
	locPort uint16
}

func newDownMap() *downMap {
	return &downMap{
		tcp:  map[de]netip.AddrPort{},
		udp:  map[de]netip.AddrPort{},
		tcpm: &sync.RWMutex{},
		udpm: &sync.RWMutex{},
	}
}

func (m *downMap) Reg(dst netip.AddrPort, proto pack.Proto, locPort uint16, src netip.AddrPort) {
	switch proto {
	case pack.TCP:
		m.RegTCP(dst, locPort, src)
	case pack.UDP:
		m.RegUDP(dst, locPort, src)
	default:
		panic(fmt.Errorf("not support proto %s", proto))
	}
}

func (m *downMap) RegUDP(dst netip.AddrPort, locPort uint16, src netip.AddrPort) {
	m.udpm.Lock()
	m.udp[de{dst, locPort}] = src
	m.udpm.Unlock()
}

func (m *downMap) RegTCP(dst netip.AddrPort, locPort uint16, src netip.AddrPort) {
	m.tcpm.Lock()
	m.tcp[de{dst, locPort}] = src
	m.tcpm.Unlock()
}

func (m *downMap) Get(dst netip.AddrPort, proto pack.Proto, locPort uint16) (src netip.AddrPort, has bool) {
	switch proto {
	case pack.TCP:
		return m.GetTCP(dst, locPort)
	case pack.UDP:
		return m.GetUDP(dst, locPort)
	default:
		panic(fmt.Errorf("not support proto %s", proto))
	}
}

func (m *downMap) GetUDP(dst netip.AddrPort, locPort uint16) (src netip.AddrPort, has bool) {
	m.udpm.RLock()
	src, has = m.udp[de{dst, locPort}]
	m.udpm.RUnlock()
	return
}

func (m *downMap) GetTCP(dst netip.AddrPort, locPort uint16) (src netip.AddrPort, has bool) {
	m.tcpm.RLock()
	src, has = m.tcp[de{dst, locPort}]
	m.tcpm.RUnlock()
	return
}

func (m *downMap) Del(dst netip.AddrPort, locPort uint16, proto pack.Proto) {
	switch proto {
	case pack.TCP:
		m.tcpm.Lock()
		delete(m.tcp, de{dst, locPort})
		m.tcpm.Unlock()
	case pack.UDP:
		m.udpm.Lock()
		delete(m.udp, de{dst, locPort})
		m.udpm.Unlock()
	default:
		panic(fmt.Errorf("not support proto %s", proto))
	}
}

func (m *downMap) Close() error {
	m.udpm.Lock()
	m.udp = map[de]netip.AddrPort{}
	m.udpm.Unlock()

	m.tcpm.Lock()
	m.tcp = map[de]netip.AddrPort{}
	m.tcpm.Unlock()

	return nil
}
