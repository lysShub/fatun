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
	ep *channel.Endpoint

	finRst chan struct{}
}

var _ link.LinkEndpoint = (*Endpoint)(nil)

func New(size int, mtu uint32, linkAddr tcpip.LinkAddress) *Endpoint {
	return &Endpoint{
		ep:     channel.New(size, mtu, linkAddr),
		finRst: make(chan struct{}),
	}
}

func (e *Endpoint) ReadContext(ctx context.Context) stack.PacketBufferPtr {
	pkt := e.ep.ReadContext(ctx)

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

	e.ep.InjectInbound(protocol, pkt)
}

func (e *Endpoint) FinRstFlag() <-chan struct{} {
	return e.finRst
}

func (e *Endpoint) Close()                                       { e.ep.Close() }
func (e *Endpoint) MTU() uint32                                  { return e.ep.MTU() }
func (e *Endpoint) MaxHeaderLength() uint16                      { return e.ep.MaxHeaderLength() }
func (e *Endpoint) LinkAddress() tcpip.LinkAddress               { return e.ep.LinkAddress() }
func (e *Endpoint) Capabilities() stack.LinkEndpointCapabilities { return e.ep.Capabilities() }
func (e *Endpoint) Attach(dispatcher stack.NetworkDispatcher)    { e.ep.Attach(dispatcher) }
func (e *Endpoint) IsAttached() bool                             { return e.ep.IsAttached() }
func (e *Endpoint) Wait()                                        { e.ep.Wait() }
func (e *Endpoint) ARPHardwareType() header.ARPHardwareType      { return e.ep.ARPHardwareType() }
func (e *Endpoint) AddHeader(pkb stack.PacketBufferPtr)          { e.ep.AddHeader(pkb) }
func (e *Endpoint) ParseHeader(pkb stack.PacketBufferPtr) bool   { return e.ep.ParseHeader(pkb) }
func (e *Endpoint) WritePackets(pkbs stack.PacketBufferList) (int, tcpip.Error) {
	panic("not support")
}
