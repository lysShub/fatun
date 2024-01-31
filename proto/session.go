package proto

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type SessionMgr interface {
	Register(s Session) (uint16, error)
	Remove(id uint16) error
}

type Session struct {
	src netip.AddrPort

	proto tcpip.TransportProtocolNumber

	dst netip.AddrPort
}

func (s *Session) encode(b []byte) error {
	if len(b) == 16 {
		saddr := netip.AddrFrom4([4]byte(b[0:]))
		sport := binary.BigEndian.Uint16(b[4:])
		s.proto = tcpip.TransportProtocolNumber(binary.BigEndian.Uint32(b[6:]))
		daddr := netip.AddrFrom4([4]byte(b[10:]))
		dport := binary.BigEndian.Uint16(b[14:])

		s.src = netip.AddrPortFrom(saddr, sport)
		s.dst = netip.AddrPortFrom(daddr, dport)
	} else if len(b) == 40 {
		saddr := netip.AddrFrom16([16]byte(b[0:]))
		sport := binary.BigEndian.Uint16(b[16:])
		s.proto = tcpip.TransportProtocolNumber(binary.BigEndian.Uint32(b[18:]))
		daddr := netip.AddrFrom16([16]byte(b[22:]))
		dport := binary.BigEndian.Uint16(b[38:])

		s.src = netip.AddrPortFrom(saddr, sport)
		s.dst = netip.AddrPortFrom(daddr, dport)
	} else {
		return fmt.Errorf("invalid session formath with length %d", len(b))
	}

	return nil
}

func (s *Session) decode() ([]byte, error) {
	var b = make([]byte, 0, 18*2+4)

	saddr := s.src.Addr()
	b = append(b, saddr.AsSlice()...)
	b = binary.BigEndian.AppendUint16(b, s.src.Port())

	b = binary.BigEndian.AppendUint32(b, uint32(s.proto))

	daddr := s.dst.Addr()
	b = append(b, daddr.AsSlice()...)
	b = binary.BigEndian.AppendUint16(b, s.dst.Port())

	if saddr.BitLen() != daddr.BitLen() || saddr.BitLen() == 0 {
		return nil, fmt.Errorf("invalid session ip address from %s to %s", saddr, daddr)
	}
	return b, nil
}

func (s *Session) Validate() bool {
	return *s != Session{}
}

func (s *Session) String() string {
	return fmt.Sprintf("%d:%s-->%s", s.proto, s.src, s.dst)
}

func (s *Session) SourceAddress() netip.AddrPort {
	return s.src
}
func (s *Session) DestinationAddress() netip.AddrPort {
	return s.dst
}
func (s *Session) TransportProtocol() tcpip.TransportProtocolNumber {
	return s.proto
}
func (s *Session) NetworkProtocol() tcpip.NetworkProtocolNumber {
	if s.src.Addr().Is4() {
		return header.IPv4ProtocolNumber
	} else {
		return header.IPv6ProtocolNumber
	}
}

func GetSession(ip []byte) Session {
	var s Session

	var (
		payload  []byte
		src, dst netip.Addr
	)
	switch header.IPVersion(ip) {
	case 4:
		iphdr := header.IPv4(ip)
		s.proto = iphdr.TransportProtocol()
		payload = iphdr.Payload()
		src = netip.AddrFrom4(iphdr.SourceAddress().As4())
		dst = netip.AddrFrom4(iphdr.DestinationAddress().As4())
	case 6:
		iphdr := header.IPv6(ip)
		s.proto = iphdr.TransportProtocol()
		payload = iphdr.Payload()
		src = netip.AddrFrom4(iphdr.SourceAddress().As4())
		dst = netip.AddrFrom4(iphdr.DestinationAddress().As4())
	default:
	}

	switch s.proto {
	case header.TCPProtocolNumber:
		tcphdr := header.TCP(payload)
		s.src = netip.AddrPortFrom(src, tcphdr.SourcePort())
		s.dst = netip.AddrPortFrom(dst, tcphdr.DestinationPort())
	case header.UDPProtocolNumber:
		udphdr := header.UDP(payload)
		s.src = netip.AddrPortFrom(src, udphdr.SourcePort())
		s.dst = netip.AddrPortFrom(dst, udphdr.DestinationPort())
	default:
	}

	return s
}
