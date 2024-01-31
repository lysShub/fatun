package rawtcp

import (
	"math/rand"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

/*
	基于raw-tcp的代理实现。

	1. 根据UrgentPointer判断： 0：控制数据报， !0: 代理数据报，其值表示session id
	2. 根据TCP flag ECE判断代理数据报： 0：tcp  1: udp
	3. 控制流必须tls
	4. 人为控制MSS，尽量DF

*/

// Segment header.IPv4 or header.IPv6, and with tcp protocol
type Segment []byte

func (p Segment) getTCPHdr() header.TCP {
	if header.IPVersion(p) == 4 {
		return header.TCP(header.IPv4(p).Payload())
	} else {
		return header.TCP(header.IPv6(p).Payload())
	}
}

func (p Segment) SetPxyseg(session uint16, proto tcpip.TransportProtocolNumber) {
	tcphdr := p.getTCPHdr()

	var delta uint16
	{
		old := tcphdr.UrgentPointer()
		delta = checksum.Combine(^old, session)
		tcphdr.SetUrgentPointer(session)
	}

	{
		f := tcphdr.Flags()
		if proto == header.TCPProtocolNumber {
			f = f ^ header.TCPFlagEce // set 0
		} else {
			f = f | header.TCPFlagEce // set 1
		}

		if old := tcphdr.Flags(); f != old {
			delta = checksum.Combine(delta, checksum.Combine(^uint16(old), uint16(f)))
			tcphdr.SetFlags(uint8(f))
		}
	}

	sum := checksum.Combine(^tcphdr.Checksum(), delta)
	tcphdr.SetChecksum(^sum)
}

func (p Segment) ResetPxyseg() {
	p.SetPxyseg(0, header.TCPProtocolNumber)
}

func (p Segment) SetCtrseg() {
	tcphdr := p.getTCPHdr()
	if old := tcphdr.UrgentPointer(); old != 0 {

		tcphdr.SetUrgentPointer(old)

		sum := checksum.Combine(checksum.Combine(^tcphdr.Checksum(), ^old), 0)
		tcphdr.SetChecksum(^sum)
	}
}

func (p Segment) IsPxyseg() bool {
	return p.getTCPHdr().UrgentPointer() != 0
}

func (p Segment) IsCtrseg() bool {
	return p.getTCPHdr().UrgentPointer() == 0
}

func (p Segment) Pxyseg() (session uint16, proto tcpip.TransportProtocolNumber) {
	tcphdr := p.getTCPHdr()

	session = tcphdr.UrgentPointer()
	if tcphdr.Flags().Contains(header.TCPFlagEce) {
		proto = header.UDPProtocolNumber
	} else {
		proto = header.TCPProtocolNumber
	}
	return
}

func UDP2TCP(ip []byte) []byte {
	var (
		b      = make([]byte, len(ip)+(header.TCPMinimumSize-header.UDPMinimumSize))
		udphdr header.UDP
		tcphdr header.TCP
	)

	switch header.IPVersion(ip) {
	case 4:
		iphdr := header.IPv4(ip)
		n := copy(b, iphdr[:iphdr.HeaderLength()])
		tcphdr = header.TCP(b[n:])
		udphdr = header.UDP(iphdr.Payload())

	case 6:
		iphdr := header.IPv6(ip)
		n := copy(b, iphdr[:header.IPv6MinimumSize])
		tcphdr = header.TCP(b[n:])
		udphdr = header.UDP(iphdr.Payload())
	default:
		return nil
	}

	tcphdr.Encode(&header.TCPFields{
		SrcPort:    udphdr.SourcePort(),
		DstPort:    udphdr.DestinationPort(),
		SeqNum:     rand.Uint32(),
		AckNum:     rand.Uint32(),
		DataOffset: header.TCPMinimumSize,
		Flags:      header.TCPFlagAck | header.TCPFlagPsh,
		WindowSize: 2510,
		Checksum:   0,
	})
	copy(tcphdr.Payload(), udphdr.Payload())

	old := checksum.Checksum(udphdr[:header.UDPMinimumSize], 0)
	new := checksum.Checksum(tcphdr[:header.TCPMinimumSize], 0)

	sum := checksum.Combine(checksum.Combine(^udphdr.Checksum(), ^old), new)
	tcphdr.SetChecksum(^sum)
	return b
}
