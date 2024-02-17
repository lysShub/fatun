package itun

import (
	"fmt"
	"net/netip"
)

const DefaultPort = 19986

//go:generate stringer -output=itun_gen.go -trimprefix=Proto -type=Proto
type Proto uint8

const (
	ICMP Proto = 1
	TCP  Proto = 6
	UDP  Proto = 17
)

func (p Proto) IsValid() bool {
	switch p {
	case TCP:
		return true
	// case ICMP, UDP: // todo: support
	// 	return true
	default:
		return false
	}
}

func (p Proto) IsICMP() bool {
	return p == ICMP || p == 58
}

type ErrInvalidProto Proto

func (e ErrInvalidProto) Error() string {
	return fmt.Sprintf("invalid transport protocol code %d", e)
}

type ErrInvalidAddr netip.Addr

func (e ErrInvalidAddr) Error() string {
	return fmt.Sprintf("invalid address %s", netip.Addr(e))
}

type Session struct {
	SrcAddr netip.AddrPort
	Proto   Proto
	DstAddr netip.AddrPort
}

func (s *Session) IsValid() bool {
	return s.SrcAddr.IsValid() &&
		s.Proto.IsValid() &&
		s.DstAddr.IsValid()
}

type ErrInvalidSession Session

func (e ErrInvalidSession) Error() string {
	return fmt.Sprintf("invalid %s session %s->%s", e.Proto, e.SrcAddr, e.DstAddr)
}
