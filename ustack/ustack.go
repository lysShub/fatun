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
	"gvisor.dev/gvisor/pkg/waiter"
)

type Ustack interface {
	Inbound(ip *relraw.Packet)
	OutboundBy(ctx context.Context, dst netip.AddrPort, ip *relraw.Packet) error
	Outbound(ctx context.Context, ip *relraw.Packet) error

	NewEndpoint(tcpip.TransportProtocolNumber, tcpip.NetworkProtocolNumber, *waiter.Queue) (tcpip.Endpoint, tcpip.Error)
}

// user mode tcp stack
type ustack struct {
	*stack.Stack

	addr  tcpip.FullAddress
	proto tcpip.NetworkProtocolNumber

	link link.Link
}

var _ Ustack = (*ustack)(nil)

// todo: set no delay
func NewUstack(addr netip.AddrPort, mtu int) (Ustack, error) {
	var u = &ustack{
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

func (u *ustack) Inbound(ip *relraw.Packet) {
	u.link.Inbound(ip)
}

// OutboundBy only use by server, read stack outbound tcp packet
func (u *ustack) OutboundBy(ctx context.Context, dst netip.AddrPort, tcp *relraw.Packet) error {
	return u.link.OutboundBy(ctx, dst, tcp)
}

// Outbound only use by client
func (u *ustack) Outbound(ctx context.Context, tcp *relraw.Packet) error {
	return u.link.Outbound(ctx, tcp)
}
