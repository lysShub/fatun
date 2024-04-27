package adapter

import (
	"net/netip"
	"sort"
	"sync"

	"github.com/lysShub/fatun/ports"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Ports struct {
	mgr *ports.PortMgr

	mu sync.RWMutex

	// sess map[Session] /* localPort */ uint16

	ports map[portKey] /* server adds */ *AddrSet
}

// NewPorts for reuse local machine port, reduce port consume
// require: one local port can be reused sessions, that has different
// destination address
func NewPorts(addr netip.Addr) *Ports {
	return &Ports{
		mgr: ports.NewPortMgr(addr),
		// sess:  make(map[Session]uint16, 16),
		ports: make(map[portKey]*AddrSet, 16),
	}
}

type AddrSet struct {
	addrs addrs
}

type addrs []netip.AddrPort

func (a addrs) Len() int           { return len(a) }
func (a addrs) Less(i, j int) bool { return less(a[i], a[j]) }
func less(a, b netip.AddrPort) bool {
	if a.Addr() != b.Addr() {
		return a.Addr().Less(b.Addr())
	} else {
		return a.Port() < b.Port()
	}
}
func (a addrs) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// idx<0 not Find
func (a *AddrSet) Find(addr netip.AddrPort) (idx int) {
	i := sort.Search(len(a.addrs), func(i int) bool {
		return !less(a.addrs[i], addr)
	})
	if i < len(a.addrs) && a.addrs[i] == addr {
		return i
	}
	return -1
}
func (a *AddrSet) Has(addr netip.AddrPort) bool {
	return a.Find(addr) >= 0
}
func (a *AddrSet) Add(addr netip.AddrPort) {
	if !a.Has(addr) {
		a.addrs = append(a.addrs, addr)
		sort.Sort(a.addrs)
	}
}
func (a *AddrSet) Del(addr netip.AddrPort) {
	i := a.Find(addr)
	if i < 0 {
		return
	}
	copy(a.addrs[i:], a.addrs[i+1:])
	a.addrs = a.addrs[:len(a.addrs)-1]
}
func (a *AddrSet) Len() int { return len(a.addrs) }

type portKey struct {
	proto     tcpip.TransportProtocolNumber
	loaclPort uint16
}

// GetPort get a local machine port
func (a *Ports) GetPort(proto tcpip.TransportProtocolNumber, dst netip.AddrPort) (port uint16, err error) {
	// try reuse alloced port,
	a.mu.RLock()
	for k, v := range a.ports {
		if k.proto != proto {
			continue
		}

		if !v.Has(dst) {
			port = k.loaclPort
			break
		}
	}
	a.mu.RUnlock()

	// alloc new port
	if port == 0 {
		switch proto {
		case header.TCPProtocolNumber:
			port, err = a.mgr.GetTCPPort()
			if err != nil {
				return 0, err
			}
		case header.UDPProtocolNumber:
			port, err = a.mgr.GetUDPPort()
			if err != nil {
				return 0, err
			}
		default:
			return 0, errors.Errorf("unknown transport protocol %d", proto)
		}
	}

	// update map
	pk := portKey{proto: proto, loaclPort: port}
	a.mu.Lock()
	if as := a.ports[pk]; as != nil {
		as.Add(dst)
	} else {
		as = &AddrSet{}
		as.Add(dst)
		a.ports[pk] = as
	}
	a.mu.Unlock()

	return port, nil
}

// todo: idle timeout delete
func (a *Ports) DelPort(proto tcpip.TransportProtocolNumber, port uint16, dst netip.AddrPort) error {
	pk := portKey{
		proto:     proto,
		loaclPort: port,
	}
	notuse := false

	a.mu.Lock()
	a.ports[pk].Del(dst)
	if a.ports[pk].Len() == 0 {
		delete(a.ports, pk)
		notuse = true
	}
	a.mu.Unlock()

	if notuse {
		switch proto {
		case header.TCPProtocolNumber:
			return a.mgr.DelTCPPort(port)
		case header.UDPProtocolNumber:
			return a.mgr.DelUDPPort(port)
		default:
			return errors.Errorf("unknown transport protocol %d", proto)
		}
	}
	return nil
}

func (a *Ports) Close() (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for pk := range a.ports {
		var e error
		switch pk.proto {
		case header.TCPProtocolNumber:
			e = a.mgr.DelTCPPort(pk.loaclPort)
		case header.UDPProtocolNumber:
			e = a.mgr.DelUDPPort(pk.loaclPort)
		default:
			panic("invalid proto")
		}
		if e != nil {
			e = err
		}
	}

	a.ports = map[portKey]*AddrSet{}
	return err
}
