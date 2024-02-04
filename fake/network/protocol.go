package network

import (
	"fmt"
	"sync/atomic"

	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type protocol struct {
	stack.NetworkProtocol

	// ctx *SeqAck
}

func NewProtocol(s *stack.Stack) stack.NetworkProtocol {
	return &protocol{
		NetworkProtocol: ipv4.NewProtocol(s),
	}
}

func NewProtocol6(s *stack.Stack) stack.NetworkProtocol {
	return &protocol{
		NetworkProtocol: ipv6.NewProtocol(s),
	}
}

func (p *protocol) NewEndpoint(nic stack.NetworkInterface, dispatcher stack.TransportDispatcher) stack.NetworkEndpoint {
	ep := p.NetworkProtocol.NewEndpoint(nic, dispatcher)
	nep, ok := ep.(NetworkEndpoint)
	if !ok {
		panic(fmt.Errorf("not support transport layer %T", ep))
	}

	// p.ctx.next = &SeqAck{}
	iep := &endpoint{
		NetworkEndpoint: nep,
		// SeqAck:          p.ctx,
	}
	// p.ctx = p.ctx.next

	return iep
}

type SeqAck struct {
	next     *SeqAck
	seq, ack atomic.Uint32
}

func (s *SeqAck) Next() *SeqAck {
	return s.next
}

func (s *SeqAck) Seq() uint32 {
	return s.seq.Load()
}

func (s *SeqAck) Ack() uint32 {
	return s.ack.Load()
}
