package maps

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun/links"
	"github.com/lysShub/fatun/peer"
	"github.com/lysShub/fatun/ports"
)

type linkManager struct {
	addr     netip.Addr
	ap       *ports.Adapter
	duration time.Duration

	uplinkMap map[links.Uplink]*port
	ttl       *links.Heap[ttlkey]
	uplinkMu  sync.RWMutex

	downlinkMap map[links.Downlink]downkey
	donwlinkMu  sync.RWMutex
}

var _ links.LinksManager = (*linkManager)(nil)

func NewLinkManager(ttl time.Duration, addr netip.Addr) *linkManager {
	return newLinkManager(ports.NewAdapter(addr), ttl)
}

func newLinkManager(ap *ports.Adapter, ttl time.Duration) *linkManager {
	return &linkManager{
		addr:     ap.Addr(),
		ap:       ap,
		duration: ttl,

		uplinkMap: map[links.Uplink]*port{},
		ttl:       links.NewHeap[ttlkey](64),

		downlinkMap: map[links.Downlink]downkey{},
	}
}

type ttlkey struct {
	s links.Uplink
	t time.Time
}

func (t ttlkey) valid() bool {
	return t.s.Process.IsValid() && t.s.Server.IsValid() && t.t != time.Time{}
}

type port atomic.Uint64

func NewPort(p uint16) *port {
	var a = &atomic.Uint64{}
	a.Store(uint64(p) << 48)
	return (*port)(a)
}
func (p *port) p() *atomic.Uint64 { return (*atomic.Uint64)(p) }
func (p *port) Idle() bool {
	d := p.p().Load()
	const flags uint64 = 0xffff000000000000

	p.p().Store(d & flags)
	return d&(^flags) == 0
}
func (p *port) Port() uint16 { return uint16(p.p().Add(1) >> 48) }

type downkey struct {
	conn       fatcp.Conn[peer.Peer]
	clientPort uint16
}

func (t *linkManager) cleanup() {
	var (
		ls     []links.Uplink
		lports []uint16
	)
	t.uplinkMu.Lock()
	for i := 0; i < t.ttl.Size(); i++ {
		i := t.ttl.Pop()
		if i.valid() && time.Since(i.t) > t.duration {
			p := t.uplinkMap[i.s]
			if p.Idle() {
				ls = append(ls, i.s)
				lports = append(lports, p.Port())
				delete(t.uplinkMap, i.s)
			} else {
				t.ttl.Put(ttlkey{i.s, time.Now()})
			}
		} else {
			t.ttl.Put(ttlkey{i.s, time.Now()})
			break
		}
	}
	t.uplinkMu.Unlock()
	if len(ls) == 0 {
		return
	}

	t.donwlinkMu.Lock()
	for i, e := range ls {
		s := links.Downlink{Server: e.Server, Proto: e.Proto, Local: netip.AddrPortFrom(t.addr, lports[i])}
		delete(t.downlinkMap, s)
	}
	t.donwlinkMu.Unlock()

	for i, e := range ls {
		t.ap.DelPort(e.Proto, lports[i], e.Server)
	}
}

func (t *linkManager) Add(s links.Uplink, conn fatcp.Conn[peer.Peer]) (localPort uint16, err error) {
	t.cleanup()

	localPort, err = t.ap.GetPort(s.Proto, s.Server)
	if err != nil {
		return 0, err
	}

	t.uplinkMu.Lock()
	t.uplinkMap[s] = NewPort(localPort)
	t.ttl.Put(ttlkey{s: s, t: time.Now()})
	t.uplinkMu.Unlock()

	t.donwlinkMu.Lock()
	t.downlinkMap[links.Downlink{
		Server: s.Server,
		Proto:  s.Proto,
		Local:  netip.AddrPortFrom(t.addr, localPort),
	}] = downkey{
		conn:       conn,
		clientPort: s.Process.Port(),
	}
	t.donwlinkMu.Unlock()

	return localPort, nil
}

// Uplink get uplink packet local port
func (t *linkManager) Uplink(s links.Uplink) (localPort uint16, has bool) {
	t.uplinkMu.RLock()
	defer t.uplinkMu.RUnlock()
	p, has := t.uplinkMap[s]
	if !has {
		return 0, false
	}
	return p.Port(), true
}

// Downlink get donwlink packet proxyer and client port
func (t *linkManager) Downlink(s links.Downlink) (conn fatcp.Conn[peer.Peer], clientPort uint16, has bool) {
	t.donwlinkMu.RLock()
	defer t.donwlinkMu.RUnlock()

	key, has := t.downlinkMap[s]
	if !has {
		return nil, 0, false
	}
	return key.conn, key.clientPort, true
}

func (t *linkManager) Close() error {
	return t.ap.Close()
}
