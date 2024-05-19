package fatun

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// Session on clinet, corresponding a transport connect
type Session struct {
	Src   netip.AddrPort
	Proto tcpip.TransportProtocolNumber
	Dst   netip.AddrPort
}

func FromIP(ip []byte) (Session, error) {
	s, _, err := fromIP(ip)
	return s, err
}

func StripIP(ip *packet.Packet) (Session, error) {
	s, n, err := fromIP(ip.Bytes())
	ip.SetHead(ip.Head() + n)
	return s, err
}

func fromIP(ip []byte) (s Session, ipHdrLen int, err error) {
	var (
		proto    tcpip.TransportProtocolNumber
		src, dst netip.Addr
		hdr      []byte
	)
	switch ver := header.IPVersion(ip); ver {
	case 4:
		ip := header.IPv4(ip)
		if n := int(ip.TotalLength()); n > len(ip) {
			return Session{}, 0, errorx.ShortBuff(n, len(ip))
		}

		src = netip.AddrFrom4(ip.SourceAddress().As4())
		dst = netip.AddrFrom4(ip.DestinationAddress().As4())
		proto = ip.TransportProtocol()
		hdr = ip.Payload()
		ipHdrLen = int(ip.HeaderLength())
	case 6:
		ip := header.IPv6(ip)
		if n := int(ip.PayloadLength()) + header.IPv6FixedHeaderSize; n > len(ip) {
			return Session{}, 0, errorx.ShortBuff(n, len(ip))
		}

		src = netip.AddrFrom16(ip.SourceAddress().As16())
		dst = netip.AddrFrom16(ip.DestinationAddress().As16())
		proto = ip.TransportProtocol()
		hdr = ip.Payload()
		ipHdrLen = header.IPv6FixedHeaderSize
	default:
		return Session{}, 0, errors.Errorf("invalid ip version %d", ver)
	}
	switch proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(hdr)
		return Session{
			Src:   netip.AddrPortFrom(src, tcp.SourcePort()),
			Proto: proto,
			Dst:   netip.AddrPortFrom(dst, tcp.DestinationPort()),
		}, ipHdrLen, nil
	case header.UDPProtocolNumber:
		udp := header.UDP(hdr)
		return Session{
			Src:   netip.AddrPortFrom(src, udp.SourcePort()),
			Proto: proto,
			Dst:   netip.AddrPortFrom(dst, udp.DestinationPort()),
		}, ipHdrLen, nil
	default:
		return Session{}, 0, errors.Errorf("not support  protocol %d", proto)
	}
}

func (s Session) IsValid() bool {
	return s.Src.IsValid() && s.Proto != 0 && s.Dst.IsValid()
}

func (s Session) String() string {
	return fmt.Sprintf("%s:%s->%s", protoStr(s.Proto), s.Src.String(), s.Dst.String())
}

func protoStr(num tcpip.TransportProtocolNumber) string {
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
