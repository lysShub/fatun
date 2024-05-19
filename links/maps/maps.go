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

type linkManager[P peer.Peer] struct {
	addr     netip.Addr
	ap       *ports.Adapter
	conns    *connManager[P]
	duration time.Duration

	uplinkMap map[links.Uplink]*port
	ttl       *links.Heap[ttlkey]
	uplinkMu  sync.RWMutex

	downlinkMap map[links.Downlink]downkey[P]
	donwlinkMu  sync.RWMutex
}

var _ links.LinksManager[peer.Peer] = (*linkManager[peer.Peer])(nil)

func NewLinkManager[P peer.Peer](ttl time.Duration, addr netip.Addr) *linkManager[P] {
	return newLinkManager[P](ports.NewAdapter(addr), newConnManager[P](), ttl)
}

func newLinkManager[P peer.Peer](ap *ports.Adapter, conns *connManager[P], ttl time.Duration) *linkManager[P] {
	return &linkManager[P]{
		addr:     ap.Addr(),
		ap:       ap,
		conns:    conns,
		duration: ttl,

		uplinkMap: map[links.Uplink]*port{},
		ttl:       links.NewHeap[ttlkey](64),

		downlinkMap: map[links.Downlink]downkey[P]{},
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

type downkey[P peer.Peer] struct {
	conn       fatcp.Conn[P]
	clientPort uint16
}

func (t *linkManager[P]) cleanup() {
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

	var conns []fatcp.Conn[P]
	t.donwlinkMu.Lock()
	for i, e := range ls {
		s := links.Downlink{Server: e.Server, Proto: e.Proto, Local: netip.AddrPortFrom(t.addr, lports[i])}
		conns = append(conns, t.downlinkMap[s].conn)
		delete(t.downlinkMap, s)
	}
	t.donwlinkMu.Unlock()

	for i, e := range ls {
		t.ap.DelPort(e.Proto, lports[i], e.Server)
	}
	for _, e := range conns {
		if e != nil {
			t.conns.Dec(e) // todo: 如果太小就close
		}
	}
}

func (t *linkManager[P]) Add(s links.Uplink, conn fatcp.Conn[P]) (localPort uint16, err error) {
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
	}] = downkey[P]{
		conn:       conn,
		clientPort: s.Process.Port(),
	}
	t.donwlinkMu.Unlock()

	return localPort, nil
}

// Uplink get uplink packet local port
func (t *linkManager[P]) Uplink(s links.Uplink) (localPort uint16, has bool) {
	t.uplinkMu.RLock()
	defer t.uplinkMu.RUnlock()
	p, has := t.uplinkMap[s]
	if !has {
		return 0, false
	}
	return p.Port(), true
}

// Downlink get donwlink packet proxyer and client port
func (t *linkManager[P]) Downlink(s links.Downlink) (conn fatcp.Conn[P], clientPort uint16, has bool) {
	t.donwlinkMu.RLock()
	defer t.donwlinkMu.RUnlock()

	key, has := t.downlinkMap[s]
	if !has {
		return nil, 0, false
	}
	return key.conn, key.clientPort, true
}

func (t *linkManager[P]) Close() error {
	return t.ap.Close()
}
