package channel

import (
	"context"

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

func (e *Endpoint) Outbound(ctx context.Context, ip *relraw.Packet) {
	pkt := e.Endpoint.ReadContext(ctx)
	if !pkt.IsNil() {
		defer pkt.DecRef()

		ip.SetLen(pkt.Size())
		b := ip.Data()

		n := 0
		for _, e := range pkt.AsSlices() {
			n += copy(b[n:], e)
		}

		// todo: optimize
		link.HandleTCPHdr([][]byte{b}, e.handle)
	} else {
		// todo: maybe return error
		ip.SetLen(0)
	}

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

	link.HandleTCPHdr([][]byte{ip.Data()}, e.handle)

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(ip.Data()),
	})
	e.Endpoint.InjectInbound(proto, pkb)
}

func (e *Endpoint) FinRstFlag() <-chan struct{} {
	return e.finRst
}
