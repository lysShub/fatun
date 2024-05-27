package fatun

import "net/netip"

func NewDefaultSender(laddr netip.AddrPort) ([]Sender, error) {
	// return NewIPSender(laddr)
	return NewETHSender(laddr)
}
