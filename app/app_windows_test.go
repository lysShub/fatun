//go:build windows
// +build windows

package app_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"testing"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/sockit/conn/tcp"
	sd "github.com/lysShub/sockit/conn/tcp/divert"
	"github.com/lysShub/sockit/packet"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestXxxx(t *testing.T) {
	divert.Load(divert.DLL)
	defer divert.Release()
	ctx := context.Background()
	fmt.Println("启动")

	// f := filter.NewMock("chrome.exe")
	f := filter.NewMock("curl.exe")
	capture, err := capture.NewCapture(f)
	require.NoError(t, err)
	defer capture.Close()

	var c *client.Client
	if true {
		cfg := &app.Config{
			Config: sconn.Config{
				PrevPackets:  pps,
				SwapKey:      sign,
				HandshakeMTU: 1460,
			},
			MTU:    1536,
			Logger: slog.NewJSONHandler(os.Stdout, nil),
		}

		raw, err := tcp.Connect(caddr, saddr, sd.Priority(1))
		require.NoError(t, err)
		conn, err := sconn.Dial(raw, &cfg.Config)
		require.NoError(t, err)

		fmt.Println("connect")

		c, err = client.NewClient(ctx, conn, cfg)
		require.NoError(t, err)
		defer c.Close()

	}

	fmt.Println("prepared")
	for {
		s, err := capture.Get(ctx)
		require.NoError(t, err)

		fmt.Println("Capture Session", s.String())

		// capture.Del(s.Session())
		// return

		err = c.AddSession(ctx, s)
		// require.NoError(t, err)

		if err != nil {
			fmt.Println("add fail", err.Error())
		} else {
			fmt.Println("AddProxy", s.String())
		}
	}
}

func Test_Capture(t *testing.T) {
	divert.Load(divert.DLL)
	defer divert.Release()

	f := filter.NewMock("curl.exe")

	capture, err := capture.NewCapture(f)
	require.NoError(t, err)
	defer capture.Close()

	for {
		s, err := capture.Get(context.Background())
		require.NoError(t, err)
		fmt.Println(s.Session().String())

		go func() {
			var b = packet.Make(0, 1536)
			var ctx = context.Background()

			for {
				err := s.Capture(ctx, b.SetHead(0))
				require.NoError(t, err)
			}
		}()
	}
}

func TestVvvv(t *testing.T) {
	divert.Load(divert.DLL)
	defer divert.Release()

	var f = `outbound and !loopback and ip and tcp.Syn`

	h, err := divert.Open(f, divert.Network, 0, divert.ReadOnly)
	require.NoError(t, err)

	var b = make([]byte, 1536)
	var ctx = context.Background()
	var addr divert.Address
	for {
		n, err := h.RecvCtx(ctx, b, &addr)
		require.NoError(t, err)

		ip := header.IPv4(b[:n])
		if ip.TransportProtocol() == header.TCPProtocolNumber {
			tcp := header.TCP(ip.Payload())
			var s = session.Session{
				Src:   netip.AddrPortFrom(netip.MustParseAddr(ip.SourceAddress().String()), tcp.SourcePort()),
				Proto: itun.TCP,
				Dst:   netip.AddrPortFrom(netip.MustParseAddr(ip.DestinationAddress().String()), tcp.DestinationPort()),
			}

			f := filter.NewMock("curl.exe")
			if f.HitOnce(s) {
				fmt.Println(s.String())
			} else {
				fmt.Println("pass", s.String())
			}
		}
	}
}
