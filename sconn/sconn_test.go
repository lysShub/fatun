package sconn

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	pkge "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type tkClient struct{}

func (c *tkClient) Token() (tk []byte, key Key, err error) {
	return []byte("hello"), Key{1: 1}, nil
}

type tkServer struct{}

func (c *tkServer) Valid(tk []byte) (key Key, err error) {
	if string(tk) == "hello" {
		return Key{1: 1}, nil
	}
	return Key{}, pkge.Errorf("invalid token")
}

func Test_Sconn(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
	)
	var pps = PrevPackets{
		header.TCP("hello"),
		header.TCP("world"),
	}

	var suit = []struct {
		pps    PrevPackets
		client SecretKey
		server SecretKey
		data   []byte
	}{
		{
			pps:    pps,
			client: &NotCryptoClient{},
			server: &NotCryptoServer{},
		},
		{
			pps:    PrevPackets{},
			client: &NotCryptoClient{},
			server: &NotCryptoServer{},
			data:   []byte("0123456789"),
		},
		{
			pps:    pps,
			client: &TokenClient{Tokener: &tkClient{}},
			server: &TokenServer{Valider: &tkServer{}},
			data:   []byte("0123456789"),
		},
	}
	var sid session.SessID = 0x23

	for _, s := range suit {
		var craw, sraw = func() (*itun.RawConn, *itun.RawConn) {
			c, s := test.NewMockRaw(
				t, header.TCPProtocolNumber,
				caddr, saddr,
				test.ValidAddr, test.ValidChecksum,
			)
			return itun.WrapRawConn(c, 1536),
				itun.WrapRawConn(s, 1536)
		}()

		go func() {
			cfg := Config{
				PrevPackets:      pps,
				HandShakeTimeout: time.Second * 3,
				SwapKey:          s.server,
			}

			ctx := cctx.WithContext(context.Background())
			sconn := Accept(ctx, sraw, &cfg)
			require.NoError(t, ctx.Err())
			defer sconn.Close()

			if len(s.data) > 0 {
				b := relraw.NewPacket(0, 1536)

				id, err := sconn.RecvSeg(ctx, b)
				require.NoError(t, err)
				require.Equal(t, sid, id)

				rdata := b.Data()
				require.Equal(t, s.data, rdata)

				err = sconn.SendSeg(ctx, b, id)
				require.NoError(t, err)
			}
		}()
		time.Sleep(time.Second)

		// client
		cfg := Config{
			PrevPackets:      pps,
			HandShakeTimeout: time.Second * 3,
			SwapKey:          s.client,
		}

		ctx := cctx.WithContext(context.Background())
		sconn := Connect(ctx, craw, &cfg)
		require.NoError(t, ctx.Err())
		defer sconn.Close()

		if len(s.data) > 0 {
			b := relraw.NewPacket(0, 1536)
			n := copy(b.Data(), s.data)
			b.SetLen(n)

			err := sconn.SendSeg(ctx, b, sid)
			require.NoError(t, err)

			b.Sets(0, 1536)
			id, err := sconn.RecvSeg(ctx, b)
			require.NoError(t, err)
			require.Equal(t, sid, id)
			require.Equal(t, s.data, b.Data())
		}
	}
}
