package link

import (
	"context"
	"net/netip"

	"github.com/lysShub/sockit/packet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// Link wrap: move tcp FIN flag to Nonce
type nofin struct {
	Link // child
}

func WrapNofin(child Link) Link {
	return &nofin{
		Link: child,
	}
}

func (n *nofin) Inbound(ip *packet.Packet) {
	decodeFakeFIN(tcphdr(ip))
	n.Link.Inbound(ip)
}
func (n *nofin) OutboundBy(ctx context.Context, dst netip.AddrPort, tcp *packet.Packet) error {
	err := n.Link.OutboundBy(ctx, dst, tcp)
	if err != nil {
		return err
	}
	encodeFakeFIN(tcp.Data())
	return nil
}
func (n *nofin) Outbound(ctx context.Context, tcp *packet.Packet) error {
	err := n.Link.Outbound(ctx, tcp)
	if err != nil {
		return err
	}
	encodeFakeFIN(tcp.Data())
	return err
}

func tcphdr(ip *packet.Packet) header.TCP {
	iph := ip.Data()
	switch header.IPVersion(iph) {
	case 4:
		return header.IPv4(iph).Payload()
	case 6:
		return iph[header.IPv6MinimumSize:]
	default:
		return nil
	}
}

const (
	fakeFinFlag              = 0b1
	fakeFinFlagOffset        = header.TCPDataOffset
	checksumDelta     uint16 = 255
)

func encodeFakeFIN(tcphdr header.TCP) bool {
	newFlags := tcphdr.Flags()
	if newFlags.Contains(header.TCPFlagFin) {

		newFlags ^= header.TCPFlagFin
		if newFlags == 0 {
			panic("todo")
		}
		tcphdr.SetFlags(uint8(newFlags))
		tcphdr[fakeFinFlagOffset] = tcphdr[fakeFinFlagOffset] ^ fakeFinFlag

		sum := ^tcphdr.Checksum()
		sum = checksum.Combine(sum, checksumDelta)
		tcphdr.SetChecksum(^sum)
		return true
	}
	return false
}

func IsFakeFIN(tcphdr header.TCP) bool {
	return tcphdr[fakeFinFlagOffset]&fakeFinFlag == fakeFinFlag
}

func decodeFakeFIN(tcphdr header.TCP) bool {
	if IsFakeFIN(tcphdr) {
		tcphdr[fakeFinFlagOffset] = tcphdr[fakeFinFlagOffset] ^ fakeFinFlag

		newFlags := uint8(tcphdr.Flags() | header.TCPFlagFin)
		tcphdr.SetFlags(newFlags)

		sum := ^tcphdr.Checksum()
		sum = checksum.Combine(sum, ^checksumDelta)
		tcphdr.SetChecksum(^sum)

		return true
	}
	return false
}
