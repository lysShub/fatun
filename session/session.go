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

func SetID(pkt *packet.Packet, id ID) {
	pkt.AllocHead(Size)
	pkt.SetHead(pkt.Head() - Size)

	b := pkt.Data()
	binary.BigEndian.PutUint16(b[idOffset1:idOffset2], uint16(id))
}

func GetID(seg *packet.Packet) ID {
	b := seg.Data()
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
	Src   netip.AddrPort
	Proto itun.Proto
	Dst   netip.AddrPort
}

func (s *Session) IsValid() bool {
	return s.Src.IsValid() &&
		s.Proto.IsValid() &&
		s.Dst.IsValid()
}

func (s *Session) String() string {
	return fmt.Sprintf("%s:%s->%s", s.Proto, s.Src, s.Dst)
}

func (s *Session) IPVersion() int {
	if s.Src.Addr().Is4() {
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

	if s.Src.Addr().Is4() {
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
