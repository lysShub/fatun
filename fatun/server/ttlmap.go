package server

import (
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/ports"
	"github.com/lysShub/fatun/fatun/server/proxyer"
	"github.com/lysShub/fatun/session"
)

type ttlmap struct {
	addr      netip.Addr
	ap        *ports.Adapter
	keepalive time.Duration

	// {clinet-addr,proto,server-addr} : local-port
	sndMap map[session.Session]uint16
	sndMu  sync.RWMutex
	ttl    *fatun.Heap[ttlkey]

	// {server-addr,proto,local-addr} : Proxyer
	rcvMap map[session.Session]rcvkey
	rcvMu  sync.RWMutex
}

func NewTTLMap(ttl time.Duration, addr netip.Addr) *ttlmap {
	return &ttlmap{
		addr:      addr,
		ap:        ports.NewAdapter(addr),
		keepalive: ttl,

		sndMap: map[session.Session]uint16{},
		ttl:    fatun.NewHeap[ttlkey](16),

		rcvMap: map[session.Session]rcvkey{},
	}
}

type ttlkey struct {
	s session.Session // {clinet-addr,proto,server-addr}
	t time.Time
}

func (t ttlkey) valid() bool {
	return t.s.IsValid() && t.t != time.Time{}
}

type rcvkey struct {
	proxyer.IProxyer
	clinetPort uint16
}

func (t *ttlmap) cleanup() {
	var (
		ss     []session.Session
		lports []uint16
	)
	t.sndMu.Lock()
	for {
		i := t.ttl.Peek()
		if i.valid() && time.Since(i.t) > t.keepalive {
			i = t.ttl.Pop()

			ss = append(ss, i.s)
			lports = append(lports, t.sndMap[i.s])
			delete(t.sndMap, i.s)
		} else {
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
		s := session.Session{Src: netip.AddrPortFrom(t.addr, lports[i]), Proto: e.Proto, Dst: e.Dst}
		pxrs = append(pxrs, t.rcvMap[s].IProxyer)
		delete(t.rcvMap, s)
	}
	t.rcvMu.Unlock()

	for i, e := range ss {
		t.ap.DelPort(e.Proto, lports[i], e.Dst)
	}
	for _, e := range pxrs {
		if e != nil {
			e.DecSession()
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
	t.sndMap[s] = locPort
	t.ttl.Put(ttlkey{s: s, t: time.Now()})
	t.sndMu.Unlock()

	t.rcvMu.Lock()
	t.rcvMap[session.Session{
		Src:   s.Dst,
		Proto: s.Proto,
		Dst:   netip.AddrPortFrom(t.addr, locPort),
	}] = rcvkey{
		IProxyer:   pxy,
		clinetPort: s.Src.Port(),
	}
	t.rcvMu.Unlock()

	return nil
}

func (t *ttlmap) Uplink(s session.Session) (localPort uint16, has bool) {
	t.sndMu.RLock()
	defer t.sndMu.RUnlock()

	localPort, has = t.sndMap[s]
	return
}

func (t *ttlmap) Downlink(s session.Session) (pxy proxyer.IProxyer, clientPort uint16, has bool) {
	t.rcvMu.RLock()
	defer t.rcvMu.RUnlock()

	key, has := t.rcvMap[s]
	if !has {
		return nil, 0, false
	}
	return key.IProxyer, key.clinetPort, true
}
