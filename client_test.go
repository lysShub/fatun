package fatun_test

import (
	"encoding/binary"
	"testing"

	"github.com/lysShub/fatun"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_UpdateMSS(t *testing.T) {

	var suits = []header.IPv4{
		{
			0x45, 0x00, 0x00, 0x34, 0x7b, 0x87, 0x40, 0x00,
			0x80, 0x06, 0x6d, 0xda, 0xc0, 0xa8, 0x2b, 0x23,
			0x77, 0x54, 0xae, 0x42, 0xcc, 0xf4, 0x01, 0xbb,
			0x06, 0xae, 0x7d, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x80, 0x02, 0xfa, 0xf0, 0x10, 0x43, 0x00, 0x00,
			0x02, 0x04, 0x05, 0xb4, 0x01, 0x03, 0x03, 0x08,
			0x01, 0x01, 0x04, 0x02,
		},
		{
			0x45, 0x00, 0x00, 0x34, 0x7b, 0x87, 0x40, 0x00,
			0x80, 0x06, 0x6d, 0xda, 0xc0, 0xa8, 0x2b, 0x23,
			0x77, 0x54, 0xae, 0x42, 0xcc, 0xf4, 0x01, 0xbb,
			0x06, 0xae, 0x7d, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x80, 0x02, 0xfa, 0xf0, 0x10, 0x43, 0x00, 0x00,
			0x01, 0x01, 0x04, 0x02, 0x01, 0x03, 0x03, 0x08,
			0x02, 0x04, 0x05, 0xb4,
		},
		{
			0x45, 0x00, 0x00, 0x34, 0x7b, 0x87, 0x40, 0x00,
			0x80, 0x06, 0x6d, 0xda, 0xc0, 0xa8, 0x2b, 0x23,
			0x77, 0x54, 0xae, 0x42, 0xcc, 0xf4, 0x01, 0xbb,
			0x06, 0xae, 0x7d, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x80, 0x02, 0xfa, 0xf0, 0x58, 0xfa, 0x00, 0x00,
			0x03, 0x03, 0x08, 0x02, 0x04, 0x05, 0xb4, 0x01,
			0x01, 0x01, 0x04, 0x02,
		},
	}

	for _, e := range suits {
		test.ValidIP(t, e)

		old := int(GetMSS(e.Payload()))
		for _, delta := range []int{-1, 0, 0x0001, 0x0100, 0x0101} {
			ip := append(header.IPv4{}, e...) // memcpy

			tcp := ip.Payload()
			fatun.UpdateTcpMssOption(tcp, delta)

			test.ValidIP(t, ip)
			new := int(GetMSS(tcp))
			require.Equal(t, old+delta, new)
		}
	}
}

func GetMSS(tcp header.TCP) uint16 {
	n := int(tcp.DataOffset())
	if n > header.TCPMinimumSize {
		for i := header.TCPMinimumSize; i < n; {
			kind := tcp[i]
			switch kind {
			case header.TCPOptionMSS:
				if i+4 <= n && tcp[i+1] == 4 {
					return binary.BigEndian.Uint16(tcp[i+2:])
				} else {
					return 0
				}
			case header.TCPOptionNOP:
				i += 1
			case header.TCPOptionEOL:
				return 0
			default:
				if i+1 < n {
					i += int(tcp[i+1])
				} else {
					return 0
				}
			}
		}
	}
	return 0
}
