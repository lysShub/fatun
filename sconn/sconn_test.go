package sconn

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Sconn(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP, test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP, test.RandPort())
	)
	var craw, sraw = func() (*itun.RawConn, *itun.RawConn) {
		c, s := test.NewMockRaw(t, header.TCPProtocolNumber, caddr, saddr)
		return itun.WrapRawConn(c, 1536),
			itun.WrapRawConn(s, 1536)
	}()

	cfg := Config{
		PrevPackets: []header.TCP{
			header.TCP("hello"),
			header.TCP("world"),
		},
		SwapKey: &NotCrypto{},
	}

	go func() {
		ctx := cctx.WithTimeout(context.Background(), time.Second*10)
		sconn := Accept(ctx, sraw, &cfg)
		require.NoError(t, ctx.Err())

		t.Log(sconn)
	}()
	time.Sleep(time.Second)

	ctx := cctx.WithTimeout(context.Background(), time.Second*10)
	sconn := Connect(ctx, craw, &cfg)
	require.NoError(t, ctx.Err())

	t.Log(sconn)
}
