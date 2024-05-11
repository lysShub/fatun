package fatun

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// Session on clinet, corresponding a transport connect
type Session struct {
	Src   netip.AddrPort
	Proto tcpip.TransportProtocolNumber
	Dst   netip.AddrPort
}

func FromIP(ip []byte) (s Session) {
	s, _ = fromIP(ip)
	return s
}

func StripIP(ip *packet.Packet) Session {
	s, n := fromIP(ip.Bytes())
	ip.SetHead(ip.Head() + n)
	return s
}

func fromIP(ip []byte) (s Session, ipHdrLen int) {
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
		ipHdrLen = int(ip.HeaderLength())
	case 6:
		ip := header.IPv6(ip)
		src = netip.AddrFrom16(ip.SourceAddress().As16())
		dst = netip.AddrFrom16(ip.DestinationAddress().As16())
		proto = ip.TransportProtocol()
		hdr = ip.Payload()
		ipHdrLen = header.IPv6FixedHeaderSize
	default:
		return Session{}, 0
	}
	switch proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(hdr)
		return Session{
			Src:   netip.AddrPortFrom(src, tcp.SourcePort()),
			Proto: proto,
			Dst:   netip.AddrPortFrom(dst, tcp.DestinationPort()),
		}, ipHdrLen
	case header.UDPProtocolNumber:
		udp := header.UDP(hdr)
		return Session{
			Src:   netip.AddrPortFrom(src, udp.SourcePort()),
			Proto: proto,
			Dst:   netip.AddrPortFrom(dst, udp.DestinationPort()),
		}, ipHdrLen
	default:
		return Session{}, 0
	}
}

func (s Session) IsValid() bool {
	return s.Src.IsValid() && s.Proto != 0 && s.Dst.IsValid()
}

func (s Session) String() string {
	return fmt.Sprintf("%s:%s->%s", ProtoStr(s.Proto), s.Src.String(), s.Dst.String())
}

func ProtoStr(num tcpip.TransportProtocolNumber) string {
	switch num {
	case header.TCPProtocolNumber:
		return "tcp"
	case header.UDPProtocolNumber:
		return "udp"
	case header.ICMPv4ProtocolNumber:
		return "icmp"
	case header.ICMPv6ProtocolNumber:
		return "icmp6"
	default:
		return fmt.Sprintf("unknown(%d)", int(num))
	}
}

func ErrInvalidSession(s Session) error {
	return errors.New(s.String())
}

type ErrSessionExist Session

func (e ErrSessionExist) Error() string {
	return fmt.Sprintf("session %s existed", Session(e).String())
}

type ErrNotSupportProto tcpip.TransportProtocolNumber

func (e ErrNotSupportProto) Error() string {
	return fmt.Sprintf("not support transport protocol %s", ProtoStr(tcpip.TransportProtocolNumber(e)))
}

func (e ErrNotSupportProto) Temporary() bool { return true }