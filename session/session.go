package session

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// todo: session

// SessionID   Payload(tcp/udp packet)
// [0, 2)      [2, n)

type ID uint16

func SetID(p *relraw.Packet, id ID) {
	p.AllocHead(Size)
	p.SetHead(p.Head() - Size)

	b := p.Data()
	binary.BigEndian.PutUint16(b[idOffset1:idOffset2], uint16(id))
}

func GetID(p *relraw.Packet) ID {
	b := p.Data()
	id := binary.BigEndian.Uint16(b[idOffset1:idOffset2])
	p.SetHead(p.Head() + Size)
	return ID(id)
}

const CtrSessID ID = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
	Size      = idOffset2
)

type Session struct {
	SrcAddr netip.AddrPort
	Proto   itun.Proto
	DstAddr netip.AddrPort
}

func (s *Session) IsValid() bool {
	return s.SrcAddr.IsValid() &&
		s.Proto.IsValid() &&
		s.DstAddr.IsValid()
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

	if s.SrcAddr.Addr().Is4() {
		minSize += header.IPv4MinimumSize
	} else {
		minSize += header.IPv6MinimumSize
	}
	return minSize
}

type ErrInvalidSession Session

func (e ErrInvalidSession) Error() string {
	return fmt.Sprintf("invalid %s session %s->%s", e.Proto, e.SrcAddr, e.DstAddr)
}
