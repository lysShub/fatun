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
		c, s := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum,
		)
		return itun.WrapRawConn(c, 1536),
			itun.WrapRawConn(s, 1536)
	}()
	var prev = []header.TCP{
		header.TCP("hello"),
		header.TCP("world"),
	}

	go func() {
		cfg := Config{
			PrevPackets: prev,
			SwapKey:     &NotCryptoClient{},
		}

		ctx := cctx.WithTimeout(context.Background(), time.Second*10)
		sconn := Accept(ctx, sraw, &cfg)
		require.NoError(t, ctx.Err())
		sconn.Close()
	}()
	time.Sleep(time.Second)

	cfg := Config{
		PrevPackets: prev,
		SwapKey:     &NotCryptoClient{},
	}

	ctx := cctx.WithTimeout(context.Background(), time.Second*10)
	sconn := Connect(ctx, craw, &cfg)
	require.NoError(t, ctx.Err())
	sconn.Close()

}
