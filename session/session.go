package session

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
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
	Src   netip.AddrPort
	Proto itun.Proto
	Dst   netip.AddrPort
}

func FromIP(ip []byte) Session {
	var (
		proto    tcpip.TransportProtocolNumber
		src, dst netip.Addr
		hdr      []byte
	)
	switch header.IPVersion(ip) {
	case 4:
		ip := header.IPv4(ip)
		src = netip.AddrFrom4(ip.SourceAddress().As4())
		dst = netip.AddrFrom4(ip.DestinationAddress().As4())
		proto = ip.TransportProtocol()
		hdr = ip.Payload()
	case 6:
		ip := header.IPv6(ip)
		src = netip.AddrFrom16(ip.SourceAddress().As16())
		dst = netip.AddrFrom16(ip.DestinationAddress().As16())
		proto = ip.TransportProtocol()
		hdr = ip.Payload()
	default:
		return Session{}
	}
	switch proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(hdr)
		return Session{
			Src:   netip.AddrPortFrom(src, tcp.SourcePort()),
			Proto: itun.TCP,
			Dst:   netip.AddrPortFrom(dst, tcp.DestinationPort()),
		}
	case header.UDPProtocolNumber:
		udp := header.UDP(hdr)
		return Session{
			Src:   netip.AddrPortFrom(src, udp.SourcePort()),
			Proto: itun.UDP,
			Dst:   netip.AddrPortFrom(dst, udp.DestinationPort()),
		}
	default:
		return Session{}
	}
}

func (s Session) IsValid() bool {
	return s.Src.IsValid() && s.Proto.Valid() && s.Dst.IsValid()
}

func (s Session) String() string {
	return fmt.Sprintf("%s:%s->%s", s.Proto, s.Src.String(), s.Dst.String())
}

func ErrInvalidSession(s Session) error {
	return errors.New(s.String())
}

func ErrExistSession(s Session) error {
	return errors.New(s.String())
}
