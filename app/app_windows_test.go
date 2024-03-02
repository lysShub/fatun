package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"

	gdivert "github.com/lysShub/divert-go"
	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/ustack/link/nofin"
	"github.com/lysShub/relraw/tcp/divert"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestXxxx(t *testing.T) {
	gdivert.Load(gdivert.DLL, gdivert.Sys)
	defer gdivert.Release()

	cfg := &client.Config{
		Sconn: sconn.Client{
			BaseConfig: sconn.BaseConfig{
				PrevPackets:      pps,
				HandShakeTimeout: ht,
			},
			SwapKey: &sconn.TokenClient{Tokener: &tkClient{}},
		},
		MTU: 1536,
	}

	ctx := context.Background()

	raw, err := divert.Connect(caddr, saddr)
	require.NoError(t, err)

	c, err := client.NewClient(ctx, raw, cfg)
	require.NoError(t, err)

	defer c.Close()

}

func Test_TCP(t *testing.T) {
	conn, err := net.DialTCP("tcp", test.TCPAddr(caddr), test.TCPAddr(saddr))
	require.NoError(t, err)

	_, err = conn.Write([]byte("hello"))
	require.NoError(t, err)

	b, err := io.ReadAll(conn)
	require.NoError(t, err)

	fmt.Println(len(b))
}

func Test_Send_Fake_IP(t *testing.T) {
	gdivert.Load(gdivert.DLL, gdivert.Sys)
	defer gdivert.Release()

	var b = header.IPv4([]byte{
		// 0x45, 0x00, 0x00, 0x28, 0x2f, 0x13, 0x40, 0x00, 0x80, 0x06, 0x00, 0x00, 0xac, 0x19, 0x20, 0x01, 0xac, 0x19, 0x26, 0x04, 0x4e, 0x12, 0x1f, 0x90, 0x93, 0x81, 0x23, 0x06, 0x02, 0x45, 0x50, 0x21, 0x50, 0x10, 0x20, 0x01, 0x9e, 0x52, 0x00, 0x00,

		0x45, 0x0, 0x0, 0x62, 0x29, 0x7a, 0x0, 0x0, 0x40, 0x6, 0xb2, 0xe4, 0xac, 0x19, 0x26, 0x4, 0xac, 0x19, 0x20, 0x1, 0x4e, 0x12, 0x1f, 0x90, 0xca, 0x42, 0x90, 0x2b, 0x2a, 0x1f, 0xf4, 0xff, 0x50, 0x18, 0xff, 0x32, 0x70, 0x7c, 0x0, 0x0, 0xd, 0xc, 0x8c, 0x1, 0x36, 0xbb, 0x81, 0x74, 0x44, 0xb0, 0x80, 0xc7, 0x7f, 0x3b, 0x77, 0xd1, 0xee, 0x3b, 0x46, 0x51, 0xec, 0xdb, 0x83, 0x7b, 0xed, 0xf3, 0x45, 0x33, 0x92, 0x39, 0x1b, 0xf5, 0xca, 0xa8, 0xa8, 0x4b, 0xb9, 0x19, 0x2a, 0x68, 0xfe, 0x55, 0xe2, 0x9a, 0x46, 0xdf, 0xf7, 0x13, 0x95, 0x60, 0xd5, 0xe4, 0xf3, 0xad, 0xc6, 0xb6, 0x8b, 0x7a,
	})

	if false {

		b.SetID(0xffff - 5)
		b.SetChecksum(^b.CalculateChecksum())

		tcp := header.TCP(b.Payload())
		tcp.SetFlags(uint8(header.TCPFlagFin | header.TCPFlagAck))
		ok := nofin.EncodeCustomFin(tcp)
		fmt.Println(ok)

		psum := header.PseudoHeaderChecksum(
			b.TransportProtocol(),
			b.SourceAddress(),
			b.DestinationAddress(),
			uint16(len(tcp)),
		)
		tcp.SetChecksum(0)
		sum := checksum.Checksum(tcp, psum)
		tcp.SetChecksum(^sum)

		test.ValidIP(t, b)
	}

	d, err := gdivert.Open("false", gdivert.NETWORK, 0, gdivert.WRITE_ONLY)
	require.NoError(t, err)
	defer d.Close()

	var addr gdivert.Address
	addr.SetOutbound(true)
	_, id, err := gdivert.Gateway(saddr.Addr())
	require.NoError(t, err)
	addr.Network().IfIdx = uint32(id)

	n, err := d.Send(b, &addr)
	require.NoError(t, err)
	t.Log(n)
}
