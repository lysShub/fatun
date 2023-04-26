package user

import (
	"itun/pack"
	"itun/server"
	"itun/server/bpf"
	"itun/util"
	"net"
	"net/netip"
	"sync"
	"unsafe"
)

type User struct {
	svc       server.Server
	proxyConn net.Conn // up-link
	pcap      *capture // down-link

	idGen *util.IdGen

	msrc    map[src]int32 // src:idx
	srcLock *sync.RWMutex
	mdst    map[dst]int32 // dst:idx
	dstLock *sync.RWMutex

	ses []session

	m *sync.RWMutex
}

// a session has unique src/dst
type src struct {
	proto   uint8
	srcPort uint16
}
type dst struct {
	proto   uint8
	locPort uint16
	dstAddr netip.AddrPort
}
type session struct {
	proto   uint8
	srcPort uint16
	locPort uint16
	dstAddr netip.AddrPort
}

func NewUser(s server.Server, conn net.Conn) *User {
	return &User{
		svc:       s,
		proxyConn: conn,

		m: &sync.RWMutex{},
	}
}

func (p *User) Do() {
	go p.uplink()
	go p.downlink()
}

func (p *User) uplink() {
	var (
		b       []byte
		n       int
		err     error
		proto   uint8
		dstIP   netip.Addr
		srcPort *uint16 = (*uint16)(unsafe.Pointer(&b[0])) // big endian
		dstPort *uint16 = (*uint16)(unsafe.Pointer(&b[2])) // big endian
		idx     int32
	)

	for {
		b = b[:cap(b)]
		if n, err = p.proxyConn.Read(b); err != nil {
			panic(err)
		} else {

			n, proto, dstIP = pack.Parse(b[:n])

			idx = p.getIdxWithSrc(src{proto, toLittle(*srcPort)})
			if idx < 0 {
				idx, err = p.newSession(proto, toLittle(*srcPort), toLittle(*dstPort), dstIP)
				if err != nil {
					panic(err)
				}
			}
			*srcPort = toBig(p.ses[idx].locPort)

			_, err = p.svc.WriteTo(proto, b[:n], dstIP)
			if err != nil {
				panic(err)
			}
		}
	}
}

// capture and write to proxyConn, need set dst-port
// for each packet, so it is stateless.
//
// todo: a goroutine for each session
func (u *User) downlink() {
	var (
		b     []byte
		n     int
		proto uint8
		addr  netip.AddrPort
		err   error
	)

	for {
		n, proto, addr, err = u.pcap.ReadFrom(b)
		if err != nil {
			panic(err)
		}
		idx := u.getIdxWithDst(dst{})
		if idx < 0 {
			continue // ports
		}
	}
}

func (p *User) newSession(proto uint8, srcPort, dstPort uint16, dstIP netip.Addr) (idx int32, err error) {
	idx = p.idGen.Get()
	for int(idx) >= len(p.ses) {
		p.ses = append(p.ses, session{})
	}

	p.ses[idx].proto = proto
	p.ses[idx].srcPort = srcPort
	p.ses[idx].dstAddr = netip.AddrPortFrom(dstIP, dstPort)
	p.ses[idx].locPort, err = p.svc.GetPort(proto, p.ses[idx].dstAddr)
	if err != nil {
		return -1, err
	}

	p.srcLock.Lock()
	p.msrc[src{proto, srcPort}] = idx
	p.srcLock.Unlock()

	p.dstLock.Lock()
	p.mdst[dst{proto, p.ses[idx].locPort, p.ses[idx].dstAddr}] = idx
	p.dstLock.Unlock()

	// set inbound capture filter
	err = p.pcap.Add(bpf.Pcap{proto, p.ses[idx].locPort, p.ses[idx].dstAddr})
	if err != nil {
		return -1, err
	}

	return idx, nil
}

func (u *User) getIdxWithSrc(s src) (idx int32) {
	has := false
	u.srcLock.RLock()
	idx, has = u.msrc[s]
	u.srcLock.RUnlock()
	if has {
		return idx
	} else {
		return -1
	}
}

func (u *User) getIdxWithDst(d dst) (idx int32) {
	has := false
	u.dstLock.RLock()
	idx, has = u.mdst[d]
	u.dstLock.RUnlock()
	if has {
		return idx
	} else {
		return -1
	}
}

func toLittle(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

func toBig(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}
