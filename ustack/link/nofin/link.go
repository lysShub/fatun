package nofin

import (
	"context"
	"sync/atomic"

	"github.com/lysShub/itun/ustack/link"
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

// Read read gvisor's outboud ip packet
func (e *Endpoint) ReadContext(ctx context.Context) stack.PacketBufferPtr {
	pkt := e.Endpoint.ReadContext(ctx) // avoid memcpy

	if !pkt.IsNil() {

		link.HandleTCPHdr(pkt.AsSlices(), e.encode)
	}

	return pkt
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

// Inject inject tcp packet to gvistor stack.
func (e *Endpoint) InjectInbound(protocol tcpip.NetworkProtocolNumber, pkt stack.PacketBufferPtr) {
	ip := pkt.AsSlices() // avoid memcpy

	link.HandleTCPHdr(ip, e.decode)

	e.Endpoint.InjectInbound(protocol, pkt)
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
