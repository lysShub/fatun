package itun

import (
	"fmt"
	"net/netip"
)

const DefaultPort = 19986

//go:generate stringer -output protocol_gen.go -trimprefix=Proto -type=Proto
type Proto uint8

const (
	ICMP   Proto = 1
	TCP    Proto = 6
	UDP    Proto = 17
	ICMPV6 Proto = 58
)

func (p Proto) IsValid() bool {
	switch p {
	case TCP, UDP:
		return true
	// case ICMP: // todo: support
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
