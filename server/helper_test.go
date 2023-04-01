package server

import (
	"net"
	"net/netip"
	"testing"
)

func TestXxx(t *testing.T) {

	var ip = net.ParseIP("8.8.8.8")

	r, err := netip.ParseAddrPort(ip.String())

	t.Log(r, err)
}
