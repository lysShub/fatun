package filter

import (
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/netkit/mapping/process"
	"github.com/pkg/errors"

	"github.com/lysShub/fatun/session"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type filter struct {
	// default rule
	defaultEnable atomic.Bool
	tcps          *tcpSyn

	dnsEnable atomic.Bool

	// process rule
	processEnable atomic.Bool
	processes     []string
	processMu     sync.RWMutex
}

func newFilter() *filter {
	return &filter{
		tcps: newAddrSyn(time.Second * 15),
	}
}

func (f *filter) Close() error { return nil }

func (f *filter) Hit(ip *packet.Packet) (bool, error) {
	id := session.FromIP(ip.Bytes())
	if debug.Debug() {
		require.True(test.T(),
			id.Src.Addr().IsPrivate() || id.Src.Addr().IsUnspecified() || id.Src.Addr().IsLoopback(),
		)
	}
	if id.Dst.Addr().IsLoopback() || id.Dst.Addr().IsMulticast() {
		return false, nil
	}

	if f.defaultEnable.Load() {
		switch id.Proto {
		case header.TCPProtocolNumber:
			var tcp header.TCP
			if id.Dst.Addr().Is4() {
				tcp = header.IPv4(ip.Bytes()).Payload()
			} else {
				tcp = header.IPv6(ip.Bytes()).Payload()
			}

			if tcp.Flags() == header.TCPFlagSyn {
				// todo: from config
				const maxsyn = 3
				n := f.tcps.Upgrade(id.Src)
				if n >= maxsyn {
					return true, nil
				}
			}
		default:
		}
	}

	if f.dnsEnable.Load() {
		const dns = 53
		if id.Dst.Port() == dns {
			return true, nil
		}
	}

	if f.processEnable.Load() {
		name, err := Global.Name(process.ID{Local: id.Src, Proto: id.Proto})
		if err != nil {
			return false, err
		} else if name == "" {
			return false, errors.WithMessage(fatun.ErrNotRecord{}, id.String())
		}

		f.processMu.RLock()
		defer f.processMu.RUnlock()
		hit := slices.Contains(f.processes, name)
		return hit, nil
	}

	return false, nil
}

func (f *filter) Add(filter string) error {
	filter = strings.TrimSpace(filter)
	switch filter {
	case DefaultFilter:
		f.defaultEnable.Store(true)
	case DNSFilter:
		f.dnsEnable.Store(true)
	default: // process name
		f.processMu.Lock()
		defer f.processMu.Unlock()

		if !slices.Contains(f.processes, filter) {
			f.processes = append(f.processes, filter)
			f.processEnable.Store(true)
		}
	}

	return nil
}
func (f *filter) Del(filter string) error {
	filter = strings.TrimSpace(filter)
	switch filter {
	case DefaultFilter:
		f.defaultEnable.Store(false)
	case DNSFilter:
		f.dnsEnable.Store(false)
	default:
		f.processMu.Lock()
		defer f.processMu.Unlock()

		f.processes = slices.DeleteFunc(f.processes,
			func(s string) bool { return s == filter },
		)
		if len(f.processes) == 0 {
			f.processEnable.Store(false)
		}
	}
	return nil
}

func (f *filter) Filters() []string {
	f.processMu.RLock()
	defer f.processMu.RUnlock()

	fs := slices.Clone(f.processes)
	if f.defaultEnable.Load() {
		fs = append([]string{DefaultFilter}, fs...)
	}
	if f.dnsEnable.Load() {
		fs = append([]string{DNSFilter}, fs...)
	}
	return fs
}

type tcpSyn struct {
	mu        sync.RWMutex
	addrs     map[netip.AddrPort]uint8
	times     *fatun.Heap[info]
	keepalive time.Duration
}

func newAddrSyn(keepalive time.Duration) *tcpSyn {
	return &tcpSyn{
		addrs:     map[netip.AddrPort]uint8{},
		times:     fatun.NewHeap[info](64),
		keepalive: keepalive,
	}
}

func (t *tcpSyn) Upgrade(addr netip.AddrPort) uint8 {
	t.mu.Lock()
	defer t.mu.Unlock()

	// clear expired addr
	i := t.times.Peek()
	for i.valid() && time.Since(i.Time) > t.keepalive {
		delete(t.addrs, t.times.Pop().AddrPort)
		i = t.times.Peek()
	}

	n, has := t.addrs[addr]
	t.addrs[addr] = n + 1
	if !has {
		t.times.Put(info{addr, time.Now()})
	}
	return n + 1
}

type info struct {
	netip.AddrPort
	time.Time
}

func (i info) valid() bool {
	return i.Time != time.Time{} && i.AddrPort.IsValid()
}
