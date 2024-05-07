package server

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/ports"
	"github.com/lysShub/fatun/fatun/server/proxyer"
	"github.com/lysShub/fatun/session"
)

type ttlmap struct {
	addr     netip.Addr
	ap       *ports.Adapter
	duration time.Duration

	// {Src: process-addr, Dst: request-addr} : local-port
	uplinkMap map[session.Session]*port
	ttl       *fatun.Heap[ttlkey]
	uplinkMu  sync.RWMutex

	// {Src: request-addr, Dst: local-addr} : Proxyer
	downlinkMap map[session.Session]rcvkey
	donwlinkMu  sync.RWMutex
}

func NewTTLMap(ttl time.Duration, addr netip.Addr) *ttlmap {
	var m = &ttlmap{
		addr:     addr,
		ap:       ports.NewAdapter(addr),
		duration: ttl,

		uplinkMap: map[session.Session]*port{},
		ttl:       fatun.NewHeap[ttlkey](16),

		downlinkMap: map[session.Session]rcvkey{},
	}

	return m
}

type ttlkey struct {
	s session.Session // {Src: process-addr, Dst: request-addr}
	t time.Time
}

func (t ttlkey) valid() bool {
	return t.s.IsValid() && t.t != time.Time{}
}

type rcvkey struct {
	proxyer    proxyer.Proxyer
	clinetPort uint16
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

func (t *ttlmap) cleanup() {
	var (
		ss     []session.Session
		lports []uint16
	)
	t.uplinkMu.Lock()
	for i := 0; i < t.ttl.Size(); i++ {
		i := t.ttl.Pop()
		if i.valid() && time.Since(i.t) > t.duration {
			p := t.uplinkMap[i.s]
			if p.Idle() {
				ss = append(ss, i.s)
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
	if len(ss) == 0 {
		return
	}

	var pxrs []proxyer.Proxyer
	t.donwlinkMu.Lock()
	for i, e := range ss {
		s := session.Session{Src: e.Dst, Proto: e.Proto, Dst: netip.AddrPortFrom(t.addr, lports[i])}
		pxrs = append(pxrs, t.downlinkMap[s].proxyer)
		delete(t.downlinkMap, s)
	}
	t.donwlinkMu.Unlock()

	for i, e := range ss {
		t.ap.DelPort(e.Proto, lports[i], e.Dst)
	}
	for i, e := range pxrs {
		if e != nil {
			e.DelSession(ss[i])
		}
	}
}

// Add add proxy session, s: {Src: process-addr, Dst: request-addr}
func (t *ttlmap) Add(s session.Session, p proxyer.Proxyer) error {
	t.cleanup()

	locPort, err := t.ap.GetPort(s.Proto, s.Dst)
	if err != nil {
		return err
	}

	t.uplinkMu.Lock()
	t.uplinkMap[s] = NewPort(locPort)
	t.ttl.Put(ttlkey{s: s, t: time.Now()})
	t.uplinkMu.Unlock()

	t.donwlinkMu.Lock()
	t.downlinkMap[session.Session{
		Src:   s.Dst,
		Proto: s.Proto,
		Dst:   netip.AddrPortFrom(t.addr, locPort),
	}] = rcvkey{
		proxyer:    p,
		clinetPort: s.Src.Port(),
	}
	t.donwlinkMu.Unlock()

	return nil
}

// Uplink get uplink packet local port, s: {Src: process-addr, Dst: request-addr}
func (t *ttlmap) Uplink(s session.Session) (localPort uint16, has bool) {
	t.uplinkMu.RLock()
	defer t.uplinkMu.RUnlock()
	p, has := t.uplinkMap[s]
	if !has {
		return 0, false
	}
	return p.Port(), true
}

// Downlink get donwlink packet proxyer and client port, s: {Src: request-addr, Dst: local-addr}
func (t *ttlmap) Downlink(s session.Session) (p proxyer.Proxyer, clientPort uint16, has bool) {
	t.donwlinkMu.RLock()
	defer t.donwlinkMu.RUnlock()

	key, has := t.downlinkMap[s]
	if !has {
		return nil, 0, false
	}
	return key.proxyer, key.clinetPort, true
}
