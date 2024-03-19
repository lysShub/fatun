package link

import (
	"context"
	"net/netip"

	"github.com/lysShub/rsocket"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Link interface {
	stack.LinkEndpoint

	Inbound(ip *rsocket.Packet)

	// OutboundBy
	OutboundBy(ctx context.Context, dst netip.AddrPort, tcp *rsocket.Packet) error
	Outbound(ctx context.Context, tcp *rsocket.Packet) error
}

func match(pkb *stack.PacketBuffer, dst netip.AddrPort) (match bool) {
	if pkb.IsNil() {
		return false
	}

	if !dst.Addr().IsValid() {
		return true
	} else {
		switch pkb.TransportProtocolNumber {
		case header.TCPProtocolNumber:
			match = dst.Port() ==
				header.TCP(pkb.TransportHeader().Slice()).DestinationPort()
		case header.UDPProtocolNumber:
			match = dst.Port() ==
				header.UDP(pkb.TransportHeader().Slice()).DestinationPort()
		case header.ICMPv4ProtocolNumber, header.ICMPv6ProtocolNumber:
			match = dst.Port() == 0
		default:
			panic("")
		}
		if !match {
			return false
		}

		switch pkb.NetworkProtocolNumber {
		case header.IPv4ProtocolNumber:
			match = dst.Addr().As4() ==
				header.IPv4(pkb.NetworkHeader().Slice()).DestinationAddress().As4()
		case header.IPv6ProtocolNumber:
			match = dst.Addr().As16() ==
				header.IPv6(pkb.NetworkHeader().Slice()).DestinationAddress().As16()
		default:
			panic("")
		}
		return match
	}
}
