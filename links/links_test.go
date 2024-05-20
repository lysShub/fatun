package links

import (
	"fmt"
	"net/netip"
	"testing"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestXxxx(t *testing.T) {

	var a = Uplink{
		netip.AddrPortFrom(netip.IPv4Unspecified(), 123),
		header.UDPProtocolNumber,
		netip.AddrPortFrom(netip.IPv4Unspecified(), 80),
	}

	str := a.String()

	fmt.Println(str)

}
