package filter

import (
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/fatun/fatun"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type defaultFilter struct {
	maxSyncs  uint8
	keepalive time.Duration

	mu    sync.RWMutex
	addrs map[netip.AddrPort]*syncs
	heap  *fatun.Heap[info]
}

// NewDefaultFiler default filter, it will proxy tcp connect while repeatedly send SYN packet maxSync times.
func NewDefaultFiler(maxSyncs uint8, keepalive time.Duration) *defaultFilter {
	return &defaultFilter{
		maxSyncs:  maxSyncs,
		keepalive: keepalive,

		addrs: map[netip.AddrPort]*syncs{},
		heap:  fatun.NewHeap[info](64),
	}
}

func (t *defaultFilter) Filter(laddr netip.AddrPort, flags header.TCPFlags) (proxy bool) {
	var new bool
	t.mu.RLock()
	s, has := t.addrs[laddr]
	if !has {
		t.addrs[laddr] = NewSyncs(t.maxSyncs)
		new = true
	} else {
		proxy = s.Inc(flags)
	}
	t.mu.RUnlock()

	if new {
		t.cleanup()
	}
	return proxy
}

func (t *defaultFilter) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for {
		e := t.heap.Pop()
		if e.valid() {
			if time.Since(e.time) > t.keepalive {
				p, has := t.addrs[e.laddr]
				if has {
					if p.Keepalive() {
						t.heap.Put(info{laddr: e.laddr, time: time.Now()})
					} else {
						delete(t.addrs, e.laddr)
					}
				}
			} else {
				t.heap.Put(info{laddr: e.laddr, time: time.Now()})
				return
			}
		} else {
			return
		}
	}
}

type syncs struct {
	limit     uint8
	count     uint8
	keepalive uint16
}

func NewSyncs(maxsyn uint8) *syncs {
	return &syncs{limit: maxsyn, count: 1}
}

func (s *syncs) Inc(flags header.TCPFlags) (proxy bool) {
	if flags == header.TCPFlagSyn {
		if s.count < s.limit {
			s.count++
		}
	} else {
		s.keepalive++
	}
	return s.count >= s.limit
}

func (s *syncs) Keepalive() bool {
	defer func() { s.keepalive = 0 }()
	return s.count >= s.limit && s.keepalive != 0
}

type info struct {
	laddr netip.AddrPort
	time  time.Time
}

func (i info) valid() bool {
	return i.time != time.Time{} && i.laddr.IsValid()
}
