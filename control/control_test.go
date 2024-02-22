package control

import (
	"context"
	"errors"
	"fmt"
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

func TestXxx(t *testing.T) {

	c, s := Sconns(t)

	t.Log(c, s)
}

func Test_Control(t *testing.T) {
	parentCtx := cctx.WithContext(context.Background())
	c, s := Sconns(t)
	t.Log("start")

	{ // server
		ctx := cctx.WithContext(parentCtx)

		raw := Serve(ctx, time.Hour, time.Hour, s, &mockServer{})

		go func() {
			seg := segment.NewSegment(1536)

			for {
				seg.Sets(0, 1536)
				err := s.RecvSeg(ctx, seg)
				if errors.Is(err, context.Canceled) {
					return
				}
				require.NoError(t, err)

				pkg := seg.Packet()
				pkg.SetHead(pkg.Head() + segment.HdrSize)

				{
					tcphdr := header.TCP(pkg.Data())
					fmt.Printf(
						"%s %d-->%d	%s\n",
						"server recv",
						tcphdr.SourcePort(), tcphdr.DestinationPort(),
						tcphdr.Flags(),
					)

				}

				raw.Inject(pkg)
			}
		}()
	}

	// client
	{
		ctx := cctx.WithContext(parentCtx)

		var raw CtrInject
		{
			raw = newUserStack(ctx, client, c)
			require.NoError(t, ctx.Err())
		}

		go func() {
			seg := segment.NewSegment(1536)

			for {
				seg.Sets(0, 1536)
				err := c.RecvSeg(ctx, seg)
				if errors.Is(err, context.Canceled) {
					return
				}
				require.NoError(t, err)

				pkg := seg.Packet()
				pkg.SetHead(pkg.Head() + segment.HdrSize)

				{
					tcphdr := header.TCP(pkg.Data())
					fmt.Printf(
						"%s %d-->%d	%s\n",
						"client recv",
						tcphdr.SourcePort(), tcphdr.DestinationPort(),
						tcphdr.Flags(),
					)
				}

				raw.Inject(pkg)
			}
		}()

		// todo: 不对, NewClient阻塞， 只有先启动协程才可能完成
		var ct *Client
		{
			tcp := connect(
				ctx, time.Hour, raw.(*Ustack),
				c.Raw().LocalAddr(), c.Raw().RemoteAddr(),
			)
			require.NoError(t, ctx.Err())

			ct = newClient(ctx, tcp)

			// ct, raw := NewClient(ctx, time.Hour, c)
			// e := ctx.Err().Error()
			// require.Empty(t, e)
		}

		ipv6 := ct.IPv6()

		t.Log(ipv6)
	}

}

func Sconns(t require.TestingT) (c, s *sconn.Conn) {
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
	var pps = sconn.PrevPackets{
		header.TCP("hello"),
		header.TCP("world"),
	}

	ctx := cctx.WithTimeout(context.Background(), time.Second*10)
	acceptCh := make(chan struct{})

	go func() {
		s = func() *sconn.Conn {
			cfg := sconn.Server{
				BaseConfig: sconn.BaseConfig{
					PrevPackets: pps,
				},
				SwapKey: &sconn.NotCryptoServer{},
			}

			sconn := sconn.Accept(ctx, time.Hour, sraw, &cfg)
			require.NoError(t, ctx.Err())

			return sconn
		}()
		close(acceptCh)
	}()

	c = func() *sconn.Conn {
		cfg := sconn.Client{
			BaseConfig: sconn.BaseConfig{
				PrevPackets: pps,
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

var _ CtrServer = (*mockServer)(nil)

func (h *mockServer) IPv6() bool { return true }
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
