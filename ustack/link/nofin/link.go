package nofin

import (
	"context"
	"sync/atomic"

	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Endpoint struct {
	*channel.Endpoint

	seq, ack atomic.Uint32
	finRst   chan struct{}
}

var _ link.LinkEndpoint = (*Endpoint)(nil)

// implement link.LinkEndpoint, the link endpoint can close
// tcp connection without FIN flag, replace by 104 bit
func New(size int, mtu uint32) *Endpoint {
	return &Endpoint{
		Endpoint: channel.New(size, mtu, ""),
		finRst:   make(chan struct{}),
	}
}

func (e *Endpoint) setSeq(seq uint32) {
	if seq > e.seq.Load() {
		e.seq.Store(seq)
	}
}

func (e *Endpoint) setAck(ack uint32) {
	e.ack.Store(ack)
}

func (e *Endpoint) SeqAck() (seq, ack uint32) {
	return e.seq.Load(), e.ack.Load()
}

func (e *Endpoint) FinRstFlag() <-chan struct{} {
	return e.finRst
}

func (e *Endpoint) Outbound(ctx context.Context, ip *relraw.Packet) {
	pkt := e.Endpoint.ReadContext(ctx) // avoid memcpy
	if !pkt.IsNil() {
		defer pkt.DecRef()

		ip.SetLen(pkt.Size())
		b := ip.Data()

		n := 0
		for _, e := range pkt.AsSlices() {
			n += copy(b[n:], e)
		}

		link.HandleTCPHdr([][]byte{b}, e.encode)
	} else {
		// todo: maybe return error
		ip.SetLen(0)
	}
}

func (e *Endpoint) encode(hdr header.TCP) (update bool) {
	if hdr.Flags().Intersects(link.FlagFinRst) {
		select {
		case <-e.finRst:
		default:
			close(e.finRst)
		}
	}

	update = EncodeCustomFin(hdr)
	e.setSeq(hdr.SequenceNumber())
	return
}

func (e *Endpoint) Inbound(ip *relraw.Packet) {
	var proto tcpip.NetworkProtocolNumber
	switch ver := header.IPVersion(ip.Data()); ver {
	case 4:
		proto = header.IPv4ProtocolNumber
	case 6:
		proto = header.IPv6ProtocolNumber
	default:
		panic(ver)
	}

	// todo: not need
	link.HandleTCPHdr([][]byte{ip.Data()}, e.decode)

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(ip.Data()),
	})
	e.Endpoint.InjectInbound(proto, pkb)
}

func (e *Endpoint) decode(hdr header.TCP) (update bool) {
	if hdr.Flags().Intersects(link.FlagFinRst) {
		select {
		case <-e.finRst:
		default:
			close(e.finRst)
		}
	}

	update = DecodeCustomFin(hdr)
	e.setAck(hdr.SequenceNumber())
	return
}
