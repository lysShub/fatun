package client_test

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/lysShub/go-divert"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type connect struct {
	cAddr, sAddr netip.AddrPort
}

func (c *connect) String() string {
	return fmt.Sprintf("%s->%s", c.cAddr, c.sAddr)
}

func TestXxx(t *testing.T) {

	// h, err := divert.Open("(ip.Protocol==6 or ipv6.NextHdr==6) and tcp.Syn", divert.LAYER_NETWORK, 11, divert.FLAG_READ_ONLY)
	h, err := divert.Open("ipv6 and tcp.Syn", divert.LAYER_NETWORK, 11, divert.FLAG_READ_ONLY)
	require.NoError(t, err)

	var m = map[connect]uint32{}

	b := make([]byte, 1536)
	for {
		b = b[:cap(b)]
		n, addr, err := h.Recv(b)
		require.NoError(t, err)

		b = b[:n]
		require.NoError(t, err)

		if !addr.Flags.IPv6() {
			hdr := header.IPv6(b)
			fmt.Println("ipv6", string(hdr.SourceAddress()))
		} else {
			hdr := header.IPv4(b)
			fmt.Println("ipv4", hdr.SourceAddress())
		}
		continue

		tcpHder := header.TCP(b[0:])

		var con = connect{
			// src: netip.AddrPortFrom(netip.AddrFrom16([16]byte(hdr.SourceAddress())), tcpHder.SourcePort()),
			// dst: netip.AddrPortFrom(netip.AddrFrom16([16]byte(hdr.Dst.To16())), tcpHder.DestinationPort()),
		}

		if seq, has := m[con]; has {

			fmt.Println("dup Syn", seq, con.String())

			delete(m, con)
		} else {
			m[con] = tcpHder.SequenceNumber()
		}

	}
}

func TestAddr(t *testing.T) {

	h, err := divert.Open("outbound", divert.LAYER_FLOW, 0, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	require.NoError(t, err)
	defer h.Close()

	for {
		_, addr, err := h.Recv(nil)
		require.NoError(t, err)

		f := addr.Flow()

		fmt.Println(f.LocalAddr(), "==>", f.RemoteAddr())
	}

}
