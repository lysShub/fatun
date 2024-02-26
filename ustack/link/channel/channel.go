package channel

import (
	"context"

	"github.com/lysShub/itun/ustack/link"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Endpoint struct {
	*channel.Endpoint

	finRst chan struct{}
}

var _ link.LinkEndpoint = (*Endpoint)(nil)

func New(size int, mtu uint32, linkAddr tcpip.LinkAddress) *Endpoint {
	return &Endpoint{
		Endpoint: channel.New(size, mtu, linkAddr),
		finRst:   make(chan struct{}),
	}
}

func (e *Endpoint) ReadContext(ctx context.Context) stack.PacketBufferPtr {
	pkt := e.Endpoint.ReadContext(ctx)

	if !pkt.IsNil() {
		link.HandleTCPHdr(pkt.AsSlices(), e.handle)
	}

	return pkt
}

func (e *Endpoint) handle(hdr header.TCP) bool {
	if hdr.Flags().Intersects(link.FlagFinRst) {
		select {
		case <-e.finRst:
		default:
			close(e.finRst)
		}
	}
	return false
}

func (e *Endpoint) InjectInbound(protocol tcpip.NetworkProtocolNumber, pkt stack.PacketBufferPtr) {
	ip := pkt.AsSlices() // avoid memcpy

	link.HandleTCPHdr(ip, e.handle)

	e.Endpoint.InjectInbound(protocol, pkt)
}

func (e *Endpoint) FinRstFlag() <-chan struct{} {
	return e.finRst
}
