package checksum

import (
	"fmt"
	"math/rand"
	"sync/atomic"

	"github.com/lysShub/fatun/links"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

/*
	约定一种传输层checksum计算方法, 为减少server的计算压力：
	uplink:
		client 使用传输层checksum的标准计算方法, 只是将src-port, PseudoHeader中的src-ip视为0。
		server 则可以根据client的计算约定, 快速求出实际的checksum。

	downlink:
		server 不计算checksum, 在client重新计算。
*/

func Client(ip *packet.Packet) (pkt *packet.Packet) {
	hdr := header.IPv4(ip.Bytes())
	if header.IPVersion(hdr) != 4 {
		panic("only support ipv4")
	}

	var t header.Transport
	switch hdr.TransportProtocol() {
	case header.TCPProtocolNumber:
		t = header.TCP(hdr.Payload())
	case header.UDPProtocolNumber:
		t = header.UDP(hdr.Payload())
	default:
		panic(fmt.Sprintf("not support protocole %d", hdr.TransportProtocol()))
	}

	srcPort := t.SourcePort()
	t.SetSourcePort(0)
	t.SetChecksum(0)
	sum := header.PseudoHeaderChecksum(
		hdr.TransportProtocol(),
		ip4zero,
		hdr.DestinationAddress(),
		uint16(len(hdr.Payload())),
	)
	t.SetChecksum(^checksum.Checksum(hdr.Payload(), sum))
	t.SetSourcePort(srcPort)

	return ip.SetHead(ip.Head() + int(hdr.HeaderLength()))
}

var ip4zero = tcpip.AddrFrom4([4]byte{})
var ip4id = atomic.Uint32{}

func init() {
	ip4id.Store(rand.Uint32())
}

func Server(pkt *packet.Packet, down links.Downlink) (ip *packet.Packet) {
	sum := checksum.Checksum(down.Local.Addr().AsSlice(), down.Local.Port())

	var t header.Transport
	switch down.Proto {
	case header.TCPProtocolNumber:
		t = header.TCP(pkt.Bytes())
	case header.UDPProtocolNumber:
		t = header.UDP(pkt.Bytes())
	default:
		panic(fmt.Sprintf("not support protocole %d", down.Proto))
	}
	t.SetChecksum(^checksum.Combine(sum, ^t.Checksum()))
	t.SetSourcePort(down.Local.Port())
	if debug.Debug() {
		test.ValidTCP(test.T(), pkt.Bytes(), header.PseudoHeaderChecksum(
			down.Proto,
			tcpip.AddrFrom4(down.Local.Addr().As4()),
			tcpip.AddrFrom4(down.Server.Addr().As4()),
			0,
		))
	}

	hdr := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())

	// notice: IPConn can set TotalLength/ID/Checksum/SrcAddr automatically, but ETHConn can't
	hdr.Encode(&header.IPv4Fields{
		TOS:            0b00001110,
		TotalLength:    uint16(len(hdr)),
		ID:             uint16(ip4id.Add(1)),
		Flags:          0,
		FragmentOffset: 0,
		TTL:            64,
		Protocol:       uint8(down.Proto),
		Checksum:       0,
		SrcAddr:        tcpip.AddrFrom4(down.Local.Addr().As4()),
		DstAddr:        tcpip.AddrFrom4(down.Server.Addr().As4()),
		Options:        nil,
	})
	hdr.SetChecksum(^hdr.CalculateChecksum())

	return pkt
}
