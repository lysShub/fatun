package adapter

import (
	"net/netip"
	"sort"
	"sync"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app"
	pkge "github.com/pkg/errors"
)

type Ports struct {
	mgr *itun.PortMgr

	mu sync.RWMutex

	// sess map[Session] /* localPort */ uint16

	ports map[portKey] /* server adds */ *AddrSet
}

// NewPorts for reuse local machine port, reduce port consume
// require: one local port can be reused sessions, that has different
// destination address
func NewPorts(addr netip.Addr) *Ports {
	return &Ports{
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
	proto     itun.Proto
	loaclPort uint16
}

// GetPort get a local machine port
func (a *Ports) GetPort(proto itun.Proto, dst netip.AddrPort) (port uint16, err error) {
	if !proto.IsValid() {
		return 0, itun.ErrInvalidProto(proto)
	} else if !dst.IsValid() {
		return 0, itun.ErrInvalidAddr(dst.Addr())
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
		case itun.TCP:
			port, err = a.mgr.GetTCPPort()
			if err != nil {
				return 0, err
			}
		case itun.UDP:
			port, err = a.mgr.GetUDPPort()
			if err != nil {
				return 0, err
			}
		default:
			return 0, pkge.Errorf("unknown transport itun code %d", proto)
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
func (a *Ports) DelPort(proto itun.Proto, port uint16, dst netip.AddrPort) error {
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
		case itun.TCP:
			return a.mgr.DelTCPPort(port)
		case itun.UDP:
			return a.mgr.DelUDPPort(port)
		default:
			return itun.ErrInvalidProto(proto)
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
		case itun.TCP:
			e = a.mgr.DelTCPPort(pk.loaclPort)
		case itun.UDP:
			e = a.mgr.DelUDPPort(pk.loaclPort)
		default:
			e = app.Join(err, itun.ErrInvalidProto(pk.proto))
		}
		err = app.Join(err, e)
	}

	a.ports = map[portKey]*AddrSet{}
	return err
}
