package ustack

import (
	"context"
	"errors"
	"net/netip"

	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

type Stack interface {
	Inbound(ip *relraw.Packet)

	// only use by server
	OutboundBy(ctx context.Context, dst netip.AddrPort, ip *relraw.Packet)
	// AcceptBy(ctx context.Context, src netip.AddrPort) net.Conn

	// only use by client
	Outbound(ctx context.Context, ip *relraw.Packet)
	// Connect(ctx context.Context, src netip.AddrPort, dst netip.AddrPort) net.Conn
}

// user mode tcp stack
type Ustack struct {
	*stack.Stack

	addr  tcpip.FullAddress
	proto tcpip.NetworkProtocolNumber

	link link.Link
}

var _ Stack = (*Ustack)(nil)

func NewUstack(addr netip.AddrPort, mtu int) (*Ustack, error) {
	var u = &Ustack{
		addr: tcpip.FullAddress{Addr: tcpip.AddrFrom4(addr.Addr().As4()), Port: addr.Port()},
	}

	var npf stack.NetworkProtocolFactory
	if addr.Addr().Is4() {
		u.proto = header.IPv4ProtocolNumber
		npf = ipv4.NewProtocol
	} else {
		u.proto = header.IPv6ProtocolNumber
		npf = ipv6.NewProtocol
	}
	u.Stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{npf},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		HandleLocal:        false,
	})
	// u.link = link.NewChan(16, mtu)
	u.link = link.NewList(16, mtu)
	if err := u.Stack.CreateNIC(nicid, u.link); err != nil {
		return nil, errors.New(err.String())
	}
	u.Stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          u.proto,
		AddressWithPrefix: u.addr.Addr.WithPrefix(),
	}, stack.AddressProperties{})
	u.Stack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: nicid},
		{Destination: header.IPv6EmptySubnet, NIC: nicid},
	})

	return u, nil
}

const nicid tcpip.NICID = 1234

func (u *Ustack) SeqAck() (uint32, uint32) {
	return u.link.SeqAck()
}

func (u *Ustack) Inbound(ip *relraw.Packet) {
	u.link.Inbound(ip)
}

func (u *Ustack) OutboundBy(ctx context.Context, dst netip.AddrPort, ip *relraw.Packet) {
	_ = u.link.OutboundBy(ctx, dst, ip)
}

func (u *Ustack) Outbound(ctx context.Context, ip *relraw.Packet) {
	_ = u.link.Outbound(ctx, ip)
}

// func (u *Ustack) Connect(ctx context.Context, src netip.AddrPort, dst netip.AddrPort) net.Conn
// func (u *Ustack) AcceptBy(ctx context.Context, src netip.AddrPort) net.Conn
