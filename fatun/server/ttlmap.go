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

	// {clinet-addr,proto,server-addr} : local-port
	sndMap map[session.Session]*port
	ttl    *fatun.Heap[ttlkey]
	sndMu  sync.RWMutex

	// {server-addr,proto,local-addr} : Proxyer
	rcvMap map[session.Session]rcvkey
	rcvMu  sync.RWMutex
}

func NewTTLMap(ttl time.Duration, addr netip.Addr) *ttlmap {
	var m = &ttlmap{
		addr:     addr,
		ap:       ports.NewAdapter(addr),
		duration: ttl,

		sndMap: map[session.Session]*port{},
		ttl:    fatun.NewHeap[ttlkey](16),

		rcvMap: map[session.Session]rcvkey{},
	}

	return m
}

type ttlkey struct {
	s session.Session // {clinet-addr,proto,server-addr}
	t time.Time
}

func (t ttlkey) valid() bool {
	return t.s.IsValid() && t.t != time.Time{}
}

type rcvkey struct {
	proxyer    proxyer.IProxyer
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
	t.sndMu.Lock()
	for i := 0; i < t.ttl.Size(); i++ {
		i := t.ttl.Pop()
		if i.valid() && time.Since(i.t) > t.duration {
			p := t.sndMap[i.s]
			if p.Idle() {
				ss = append(ss, i.s)
				lports = append(lports, p.Port())
				delete(t.sndMap, i.s)
			} else {
				t.ttl.Put(ttlkey{i.s, time.Now()})
			}
		} else {
			t.ttl.Put(ttlkey{i.s, time.Now()})
			break
		}
	}
	t.sndMu.Unlock()
	if len(ss) == 0 {
		return
	}

	var pxrs []proxyer.IProxyer
	t.rcvMu.Lock()
	for i, e := range ss {
		s := session.Session{Src: e.Dst, Proto: e.Proto, Dst: netip.AddrPortFrom(t.addr, lports[i])}
		pxrs = append(pxrs, t.rcvMap[s].proxyer)
		delete(t.rcvMap, s)
	}
	t.rcvMu.Unlock()

	for i, e := range ss {
		t.ap.DelPort(e.Proto, lports[i], e.Dst)
	}
	for i, e := range pxrs {
		if e != nil {
			e.DecSession(ss[i])
		}
	}
}

func (t *ttlmap) Add(s session.Session, pxy proxyer.IProxyer) error {
	t.cleanup()

	locPort, err := t.ap.GetPort(s.Proto, s.Dst)
	if err != nil {
		return err
	}

	t.sndMu.Lock()
	t.sndMap[s] = NewPort(locPort)
	t.ttl.Put(ttlkey{s: s, t: time.Now()})
	t.sndMu.Unlock()

	t.rcvMu.Lock()
	t.rcvMap[session.Session{
		Src:   s.Dst,
		Proto: s.Proto,
		Dst:   netip.AddrPortFrom(t.addr, locPort),
	}] = rcvkey{
		proxyer:    pxy,
		clinetPort: s.Src.Port(),
	}
	t.rcvMu.Unlock()

	return nil
}

func (t *ttlmap) Uplink(s session.Session) (localPort uint16, has bool) {
	t.sndMu.RLock()
	defer t.sndMu.RUnlock()
	p, has := t.sndMap[s]
	if !has {
		return 0, false
	}
	return p.Port(), true
}

func (t *ttlmap) Downlink(s session.Session) (pxy proxyer.IProxyer, clientPort uint16, has bool) {
	t.rcvMu.RLock()
	defer t.rcvMu.RUnlock()

	key, has := t.rcvMap[s]
	if !has {
		return nil, 0, false
	}
	return key.proxyer, key.clinetPort, true
}
