package server

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"sync"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/protocol"
)

type PortAdapter struct {
	mgr *itun.PortMgr

	mu sync.RWMutex

	// sess map[Session] /* localPort */ uint16

	ports map[portKey] /* server adds */ *AddrSet
}

// NewPortAdapter for reuse local machine port, reduce port consume
// require: one local port can be reused sessions, that has different
// destination address
func NewPortAdapter(addr netip.Addr) *PortAdapter {
	return &PortAdapter{
		mgr: itun.NewPortMgr(addr),
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
	proto     protocol.Proto
	loaclPort uint16
}

// GetPort get a local machine port
func (a *PortAdapter) GetPort(proto protocol.Proto, dst netip.AddrPort) (port uint16, err error) {
	if !proto.IsValid() {
		return 0, protocol.ErrInvalidProto(proto)
	} else if !dst.IsValid() {
		return 0, protocol.ErrInvalidAddr(dst.Addr())
	}

	if proto.IsICMP() {
		return 0, nil
	}

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
		case protocol.TCP:
			port, err = a.mgr.GetTCPPort()
			if err != nil {
				return 0, err
			}
		case protocol.UDP:
			port, err = a.mgr.GetUDPPort()
			if err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("unknown transport protocol code %d", proto)
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
func (a *PortAdapter) DelPort(proto protocol.Proto, port uint16, dst netip.AddrPort) error {
	if proto.IsICMP() {
		return nil
	}

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
		case protocol.TCP:
			return a.mgr.DelTCPPort(port)
		case protocol.UDP:
			return a.mgr.DelUDPPort(port)
		default:
			return protocol.ErrInvalidProto(proto)
		}
	}
	return nil
}

func (a *PortAdapter) Close() (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for pk := range a.ports {
		var e error
		switch pk.proto {
		case protocol.TCP:
			e = a.mgr.DelTCPPort(pk.loaclPort)
		case protocol.UDP:
			e = a.mgr.DelUDPPort(pk.loaclPort)
		default:
			e = errors.Join(err, protocol.ErrInvalidProto(pk.proto))
		}
		err = errors.Join(err, e)
	}

	a.ports = map[portKey]*AddrSet{}
	return err
}
