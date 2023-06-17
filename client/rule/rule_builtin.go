package rule

import (
	"fmt"
	"itun/client/priority"
	"time"

	"github.com/lysShub/go-divert"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const BuiltinRule string = "builtin"

type builtinRuler struct {
	baseRuler

	m         map[tcpId]rec
	eventTime time.Time
}

type rec struct {
	count   int8
	delTime time.Time
	proxy   bool
}

func newBuiltinRule(ch chan string) (Rule, error) {
	var err error
	var r = &builtinRuler{eventTime: time.Now()}
	r.ch = ch
	r.m = make(map[tcpId]rec, 8)

	var filter = "!loopback and tcp.Syn"
	// TODO: LAYER_NETWORK_FORWARDED with ifIdx
	r.listener, err = divert.Open(filter, divert.LAYER_NETWORK, priority.DefaultBuiltinRulePriority, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		return nil, err
	}

	go r.do()
	return r, nil
}

var _ Rule = &builtinRuler{}

type tcpId struct {
	// proto is tcp
	laddr, raddr string
	lport, rport uint16
	seq          uint32
}

func (id tcpId) proxy() string {
	var f = fmt.Sprintf("tcp and localAddr=%s and remoteAddr=%s and localPort=%d and remotePort=%d", id.laddr, id.raddr, id.lport, id.rport)
	return f
}

func (r *builtinRuler) do() {
	var err error
	var b = make([]byte, 1536)
	var n int
	var addr divert.Address
	var id tcpId
	for !r.done.Load() {
		b = b[:cap(b)]
		n, addr, err = r.listener.Recv(b)
		if err != nil {
			panic(err)
		}

		b = b[:n]
		if addr.IPv6() {
			const ipv6HdrLen = 40
			ipHdr := header.IPv6(b)
			tcpHdr := header.TCP(b[ipv6HdrLen:])
			id = tcpId{
				laddr: ipHdr.SourceAddress().String(),
				lport: tcpHdr.SourcePort(),
				raddr: ipHdr.DestinationAddress().String(),
				rport: tcpHdr.DestinationPort(),
				seq:   tcpHdr.SequenceNumber(),
			}
		} else {
			ipHdr := header.IPv4(b)
			tcpHdr := header.TCP(b[ipHdr.HeaderLength():])
			id = tcpId{
				laddr: ipHdr.SourceAddress().String(),
				lport: tcpHdr.SourcePort(),
				raddr: ipHdr.DestinationAddress().String(),
				rport: tcpHdr.DestinationPort(),
				seq:   tcpHdr.SequenceNumber(),
			}
		}

		rec := r.m[id]
		if rec.count < 2 {
			if rec.count == 0 {
				rec.delTime = time.Now().Add(time.Second * 10)
			}
			rec.count++
			r.m[id] = rec
		} else {
			if !rec.proxy {
				select {
				case r.ch <- id.proxy():
				default:
					fmt.Println("rules channel is full")
				}
				rec.proxy = true
				r.m[id] = rec
			}
		}

		if time.Since(r.eventTime) > time.Second {
			for k, v := range r.m {
				if time.Now().After(v.delTime) {
					delete(r.m, k)
				}
			}
			r.eventTime = time.Now()
		}
	}
}

func (r *builtinRuler) Close() error {
	r.done.Store(true)
	r.listener.Shutdown(divert.SHUTDOWN_RECV)
	return r.listener.Close()
}
