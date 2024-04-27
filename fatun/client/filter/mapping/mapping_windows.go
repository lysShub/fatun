//go:build windows
// +build windows

package mapping

import (
	"cmp"
	"net/netip"
	"os"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/lysShub/fatun/session"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type mapping struct {
	tcp *table
	udp *table

	closeErr atomic.Pointer[error]
}

var _ Mapping = (*mapping)(nil)

func newMapping() (*mapping, error) {
	var m = &mapping{
		tcp: newTable(header.TCPProtocolNumber),
		udp: newTable(header.UDPProtocolNumber),
	}
	// m.query(Endpoint{Proto: header.UDPProtocolNumber})
	// m.query(Endpoint{Proto: header.TCPProtocolNumber})
	return m, nil
}

func (m *mapping) close(cause error) error {
	if m.closeErr.CompareAndSwap(nil, &os.ErrClosed) {

		if cause != nil {
			m.closeErr.Store(&cause)
		}
		return cause
	}
	return *m.closeErr.Load()
}

func (m *mapping) query(ep Endpoint) (elem, error) {
	switch ep.Proto {
	case header.TCPProtocolNumber:
		e := m.tcp.Query(ep.Addr)
		if e.valid() {
			return e, nil
		}

		if err := m.tcp.Upgrade(); err != nil {
			return elem{}, err
		}

		e = m.tcp.Query(ep.Addr)
		if e.valid() {
			return e, nil
		}
		return elem{}, nil // not record
	case header.UDPProtocolNumber:
		e := m.udp.Query(ep.Addr)
		if e.valid() {
			return e, nil
		}

		if err := m.udp.Upgrade(); err != nil {
			return elem{}, err
		}

		e = m.udp.Query(ep.Addr)
		if e.valid() {
			return e, nil
		}
		return elem{}, nil // not record
	default:
		return elem{}, errors.Errorf("not support protocol %s", session.ProtoStr(ep.Proto))
	}

}

func (m *mapping) Name(ep Endpoint) (string, error) {
	if e, err := m.query(ep); err != nil {
		return "", err
	} else {
		return e.name, nil
	}
}

func (m *mapping) Pid(ep Endpoint) (uint32, error) {
	if e, err := m.query(ep); err != nil {
		return 0, err
	} else {
		return e.pid, nil
	}
}

func (m *mapping) Pids() []uint32 {
	var pids = make([]uint32, 0, m.tcp.size()+m.udp.size())
	pids = m.tcp.Pids(pids)
	pids = m.udp.Pids(pids)
	slices.Sort(pids)
	return slices.Compact(pids)
}

func (m *mapping) Names() []string {
	var names = make([]string, 0, m.tcp.size()+m.udp.size())
	names = m.tcp.Names(names)
	names = m.udp.Names(names)
	slices.Sort(names)
	return slices.Compact(names)
}
func (m *mapping) Close() error { return m.close(nil) }

type table struct {
	mu    sync.RWMutex
	proto tcpip.TransportProtocolNumber
	elems []elem // asc by port
}

func newTable(proto tcpip.TransportProtocolNumber) *table {
	return &table{
		proto: proto,
		elems: make([]elem, 0, 16),
	}
}

func (t *table) Query(addr netip.AddrPort) elem {
	t.mu.RLock()
	defer t.mu.RUnlock()

	port := addr.Port()
	n := len(t.elems)
	i := sort.Search(n, func(i int) bool {
		return t.elems[i].port >= port
	})
	for ; i < n; i++ {
		if t.elems[i].port == port {
			if t.elems[i].addr == addr.Addr() || t.elems[i].addr.IsUnspecified() {
				return t.elems[i]
			}
		} else {
			break
		}
	}
	return elem{}
}

func (t *table) Upgrade() error {
	ss, err := net.Connections(session.ProtoStr(t.proto))
	if err != nil {
		return errors.WithStack(err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.elems = t.elems[:0]

	for _, e := range ss {
		name, err := (&process.Process{Pid: e.Pid}).Name()
		if err != nil {
			continue
		}
		t.elems = append(t.elems, elem{
			port: uint16(e.Laddr.Port),
			pid:  uint32(e.Pid),
			addr: netip.MustParseAddr(e.Laddr.IP),
			name: name,
		})
	}

	slices.SortFunc(t.elems, func(a, b elem) int {
		return cmp.Compare(a.port, b.port)
	})

	return nil
}

func (t *table) Pids(pids []uint32) []uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, e := range t.elems {
		pids = append(pids, e.pid)
	}
	return pids
}
func (t *table) Names(names []string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, e := range t.elems {
		names = append(names, e.name)
	}
	return names
}
func (t *table) size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.elems)
}

type elem struct {
	port uint16
	pid  uint32
	addr netip.Addr
	name string
}

func (e elem) valid() bool { return e.pid != 0 }
