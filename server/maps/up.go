package maps

import (
	"fmt"
	"itun/pack"
	"itun/server/ports"
	"net/netip"
	"sync"
	"sync/atomic"
)

type upMap struct {
	ports *ports.Ports

	tcp     map[ue]uint16
	tcpRefs [65536]atomic.Int32
	tcpm    *sync.RWMutex

	udp     map[ue]uint16
	udpRefs [65536]atomic.Int32
	udpm    *sync.RWMutex

	// DOTO: icmp not need port
}

type ue struct {
	src netip.AddrPort
	dst netip.AddrPort
}

func newUpMap(laddr netip.Addr) (*upMap, error) {
	var r = &upMap{
		tcp:  map[ue]uint16{},
		tcpm: &sync.RWMutex{},

		udp:  map[ue]uint16{},
		udpm: &sync.RWMutex{},
	}

	var err error
	r.ports, err = ports.NewPorts(laddr)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (m *upMap) Get(src, dst netip.AddrPort, proto pack.Proto) (locPort uint16, newLocPort bool, err error) {
	switch proto {
	case pack.TCP:
		return m.GetTCP(src, dst)
	case pack.UDP:
		return m.GetUDP(src, dst)
	default:
		return 0, false, fmt.Errorf("not support proto %s", proto)
	}
}

func (m *upMap) GetUDP(src, dst netip.AddrPort) (locPort uint16, newLocPort bool, err error) {
	defer func() {
		if err == nil {
			if newLocPort {
				m.udpRefs[locPort].Store(1)
			} else {
				m.udpRefs[locPort].Add(1)
			}
		}
	}()

	return m.rawGetUDP(src, dst)
}

func (m upMap) rawGetUDP(src, dst netip.AddrPort) (locPort uint16, newLocPort bool, err error) {
	var has bool

	m.udpm.RLock()
	locPort, has = m.udp[ue{src, dst}]
	m.udpm.RUnlock()
	if !has {
		locPort, err = m.ports.GetPort(pack.UDP)
		if err != nil {
			return 0, false, err
		}
		m.udpm.Lock()
		m.udp[ue{src, dst}] = locPort
		m.udpm.Unlock()
	}
	return locPort, !has, nil
}

func (m *upMap) GetTCP(src, dst netip.AddrPort) (locPort uint16, newLocPort bool, err error) {
	defer func() {
		if err == nil {
			if newLocPort {
				m.tcpRefs[locPort].Store(1)
			} else {
				m.tcpRefs[locPort].Add(1)
			}
		}
	}()

	return m.rawGetTCP(src, dst)
}

func (m *upMap) rawGetTCP(src, dst netip.AddrPort) (locPort uint16, new bool, err error) {
	var has bool

	m.tcpm.RLock()
	locPort, has = m.tcp[ue{src, dst}]
	m.tcpm.RUnlock()
	if !has {
		locPort, err = m.ports.GetPort(pack.TCP)
		if err != nil {
			return 0, false, err
		}
		m.tcpm.Lock()
		m.tcp[ue{src, dst}] = locPort
		m.tcpm.Unlock()
	}

	return locPort, !has, nil
}

func (m *upMap) Rel(src, dst netip.AddrPort, proto pack.Proto) error {
	switch proto {
	case pack.TCP:
		return m.RelTCP(src, dst)
	case pack.UDP:
		return m.RelUDP(src, dst)
	default:
		return fmt.Errorf("not support proto %s", proto)
	}
}

func (m *upMap) RelUDP(src, dst netip.AddrPort) error {
	m.udpm.RLock()
	locPort, has := m.udp[ue{src, dst}]
	m.udpm.RUnlock()

	if has {
		m.udpm.Lock()
		delete(m.udp, ue{src, dst})
		m.udpm.Unlock()

		if m.udpRefs[locPort].Add(-1) == 0 {
			return m.ports.ClosePort(pack.UDP, locPort)
		}
	}
	return nil
}

func (m *upMap) RelTCP(src, dst netip.AddrPort) error {
	m.tcpm.RLock()
	locPort, has := m.tcp[ue{src, dst}]
	m.tcpm.RUnlock()

	if has {
		m.tcpm.Lock()
		delete(m.tcp, ue{src, dst})
		m.tcpm.Unlock()

		if m.tcpRefs[locPort].Add(-1) == 0 {
			return m.ports.ClosePort(pack.TCP, locPort)
		}
	}

	return nil
}

func (m *upMap) Close() (err error) {
	err = m.ports.Close()

	m.tcpm.Lock()
	m.tcp = map[ue]uint16{}
	m.tcpm.Unlock()

	m.udpm.Lock()
	m.udp = map[ue]uint16{}
	m.udpm.Unlock()

	return err
}
