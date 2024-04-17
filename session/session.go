package session

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// todo: session

// SessionID   Payload(tcp/udp packet)
// [0, 2)      [2, n)

type ID uint16

func Encode(pkt *packet.Packet, id ID) {
	pkt.Attach(binary.BigEndian.AppendUint16(nil, uint16(id)))
}

func Decode(seg *packet.Packet) ID {
	b := seg.Bytes()
	id := binary.BigEndian.Uint16(b[idOffset1:idOffset2])
	seg.SetHead(seg.Head() + Size)
	return ID(id)
}

func ErrInvalidID(id ID) error {
	return errors.Errorf("invalid session id %d", id)
}

func ErrExistID(id ID) error {
	return errors.Errorf("exist session id %d", id)
}

const CtrSessID ID = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
	Size      = idOffset2
)

type Session struct {
	SrcAddr netip.Addr
	SrcPort uint16
	Proto   itun.Proto
	DstAddr netip.Addr
	DstPort uint16
}

func FromIP(ip []byte) Session {
	var (
		s     Session
		iphdr header.Network
		hdr   header.Transport
	)
	switch header.IPVersion(ip) {
	case 4:
		iphdr = header.IPv4(ip)
		s.SrcAddr = netip.AddrFrom4(iphdr.SourceAddress().As4())
		s.DstAddr = netip.AddrFrom4(iphdr.DestinationAddress().As4())
	case 6:
		iphdr = header.IPv6(ip)
		s.SrcAddr = netip.AddrFrom16(iphdr.SourceAddress().As16())
		s.DstAddr = netip.AddrFrom16(iphdr.DestinationAddress().As16())
	default:
		return Session{}
	}
	switch iphdr.TransportProtocol() {
	case header.TCPProtocolNumber:
		s.Proto = itun.TCP
		hdr = header.TCP(iphdr.Payload())
	case header.UDPProtocolNumber:
		s.Proto = itun.UDP
		hdr = header.UDP(iphdr.Payload())
	default:
		return Session{}
	}
	s.SrcPort = hdr.SourcePort()
	s.DstPort = hdr.DestinationPort()
	return s
}

func (s Session) IsValid() bool {
	return s.SrcAddr.IsValid() &&
		s.Proto.IsValid() &&
		s.DstAddr.IsValid()
}

func (s Session) Src() netip.AddrPort {
	return netip.AddrPortFrom(s.SrcAddr, s.SrcPort)
}

func (s Session) Dst() netip.AddrPort {
	return netip.AddrPortFrom(s.DstAddr, s.DstPort)
}

func (s Session) String() string {
	return fmt.Sprintf("%s:%s->%s", s.Proto, s.Src(), s.Dst())
}

func (s Session) IPVersion() int {
	if s.SrcAddr.Is4() {
		return 4
	}
	return 6
}

func (s *Session) MinPacketSize() int {
	var minSize int
	switch s.Proto {
	case itun.TCP:
		minSize += header.TCPMinimumSize
	case itun.UDP:
		minSize += header.UDPMinimumSize
	case itun.ICMP:
		minSize += header.ICMPv4MinimumSize
	case itun.ICMPV6:
		minSize += header.ICMPv6MinimumSize
	default:
		panic("")
	}

	if s.SrcAddr.Is4() {
		minSize += header.IPv4MinimumSize
	} else {
		minSize += header.IPv6MinimumSize
	}
	return minSize
}

func ErrInvalidSession(s Session) error {
	return errors.New(s.String())
}

func ErrExistSession(s Session) error {
	return errors.New(s.String())
}
