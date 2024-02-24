package control

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Control(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP, test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP, test.RandPort())
	)
	parentCtx := cctx.WithContext(context.Background())
	c, s := CreateSconns(t, caddr, saddr)

	// server
	go func() {
		ctx := cctx.WithContext(parentCtx)

		ctr, err := NewController(saddr, caddr, s.Raw().MTU())
		require.NoError(t, err)
		defer ctr.close()

		go ctr.OutboundService(ctx, s)
		go func() {
			seg := segment.NewSegment(1536)
			for {
				seg.Sets(0, 1536)
				err := s.RecvSeg(ctx, seg)
				if errors.Is(err, context.Canceled) {
					return
				}
				require.NoError(t, err)

				ctr.Inbound(seg)
			}
		}()

		Serve(ctx, ctr, &mockServer{})
	}()

	// client
	{
		ctx := cctx.WithContext(parentCtx)

		ctr, err := NewController(caddr, saddr, c.Raw().MTU())
		require.NoError(t, err)
		defer ctr.close()

		go ctr.OutboundService(ctx, c)
		go func() {
			seg := segment.NewSegment(1536)
			for {
				seg.Sets(0, 1536)
				err := c.RecvSeg(ctx, seg)
				if errors.Is(err, context.Canceled) {
					return
				}
				require.NoError(t, err)

				ctr.Inbound(seg)
			}
		}()

		client := Dial(ctx, ctr)
		require.NoError(t, ctx.Err())
		defer client.Close()

		ipv6, err := client.IPv6(ctx)
		require.NoError(t, err)
		require.True(t, ipv6)
	}
}

func Test_Control_Close(t *testing.T) {
	t.Skip("todo")
}

func CreateSconns(t require.TestingT, caddr, saddr netip.AddrPort) (c, s *sconn.Conn) {
	var craw, sraw = func() (*itun.RawConn, *itun.RawConn) {
		c, s := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum,
		)
		return itun.WrapRawConn(c, 1536),
			itun.WrapRawConn(s, 1536)
	}()
	var pps = sconn.PrevPackets{
		header.TCP("hello"),
		header.TCP("world"),
	}

	ctx := cctx.WithTimeout(context.Background(), time.Second*10)
	defer ctx.Cancel(nil)
	acceptCh := make(chan struct{})

	go func() {
		s = func() *sconn.Conn {
			cfg := sconn.Server{
				BaseConfig: sconn.BaseConfig{
					PrevPackets:      pps,
					HandShakeTimeout: time.Hour,
				},
				SwapKey: &sconn.NotCryptoServer{},
			}

			sconn := sconn.Accept(ctx, sraw, &cfg)
			require.NoError(t, ctx.Err())

			return sconn
		}()
		close(acceptCh)
	}()

	c = func() *sconn.Conn {
		cfg := sconn.Client{
			BaseConfig: sconn.BaseConfig{
				PrevPackets:      pps,
				HandShakeTimeout: time.Hour,
			},
			SwapKey: &sconn.NotCryptoClient{},
		}

		sconn := sconn.Connect(ctx, craw, &cfg)
		require.NoError(t, ctx.Err())
		return sconn
	}()

	<-acceptCh
	require.NoError(t, ctx.Err())

	return c, s
}

type mockServer struct {
	InitedCfg bool
}

var _ SrvHandler = (*mockServer)(nil)

func (h *mockServer) IPv6() bool {
	return true
}
func (h *mockServer) EndConfig() {
	h.InitedCfg = true
}
func (h *mockServer) AddTCP(addr netip.AddrPort) (uint16, error) {
	return 1, nil
}
func (h *mockServer) DelTCP(id uint16) error {
	return nil
}
func (h *mockServer) AddUDP(addr netip.AddrPort) (uint16, error) {
	return 2, nil
}
func (h *mockServer) DelUDP(id uint16) error {
	return nil
}
func (h *mockServer) PackLoss() float32 { return 0.01 }
func (h *mockServer) Ping()             {}
