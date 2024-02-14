package itun

import "net/netip"

type Session struct {
	Proto Proto

	// proxy connect's destination address
	Server netip.AddrPort
}
