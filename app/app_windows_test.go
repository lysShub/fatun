package app_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"

	gdivert "github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw/tcp/divert"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	gdivert.Load(gdivert.DLL, gdivert.Sys)
	defer gdivert.Release()

	cfg := &client.Config{
		Config: config.Config{
			PrevPackets:      pps,
			HandShakeTimeout: ht,
			SwapKey:          &crypto.TokenClient{Tokener: &tkClient{}},
		},
		MTU: 1536,
	}

	ctx := context.Background()

	raw, err := divert.Connect(caddr, saddr)
	require.NoError(t, err)

	c, err := client.NewClient(ctx, raw, cfg)
	require.NoError(t, err)

	err = c.Handshake()
	require.NoError(t, err)
	defer c.Close()

	err = c.AddProxy(session.Session{
		Src:   netip.AddrPortFrom(netip.AddrFrom4([4]byte{192, 168, 43, 35}), 12345),
		Proto: itun.TCP,
		Dst:   netip.AddrPortFrom(netip.AddrFrom4([4]byte{8, 8, 8, 8}), 80),
	})
	require.NoError(t, err)

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
