//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/sockit/conn/tcp"
	dconn "github.com/lysShub/sockit/conn/tcp/divert"
	"github.com/lysShub/sockit/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

var (
	caddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{
		// 172, 25, 32, 1,
		172, 24, 128, 1,
	}), 19986)

	saddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{
		// 172, 25, 38, 4,
		172, 24, 131, 26,
	}), 8080)
)

var sign = &sconn.Sign{
	Sign: []byte("0123456789abcdef"),
	Parser: func(sign []byte) (crypto.Key, error) {
		return crypto.Key{9: 1}, nil
	},
}

var pps = sconn.PrevPackets{
	header.TCP("hello"),
	header.TCP("world"),
}

func main() {
	divert.Load(divert.DLL)
	defer divert.Release()

	var _ = header.TCPProtocolNumber
	var t = test.T()

	ctx := context.Background()
	fmt.Println("启动")

	var c *client.Client
	{
		raw, err := tcp.Connect(caddr, saddr, dconn.Priority(1))
		require.NoError(t, err)

		cfg := &app.Config{
			Config: sconn.Config{
				PrevPackets:  pps,
				SwapKey:      sign,
				HandshakeMTU: 1460,
			},
			MTU:    1536,
			Logger: slog.NewJSONHandler(os.Stdout, nil),
		}
		// wraw, err := test.WrapPcap(raw, "raw.pcap")
		// require.NoError(t, err)
		conn, err := sconn.Dial(raw, &cfg.Config)
		require.NoError(t, err)

		fmt.Println("connect")

		c, err = client.NewClient(ctx, conn, cfg)
		require.NoError(t, err)
		defer c.Close()
	}

	f, err := filter.New()
	require.NoError(t, err)
	capture, err := capture.NewCapture(f, &capture.Config{
		Logger:   slog.New(slog.NewJSONHandler(os.Stderr, nil)),
		Priority: 1,
		Mtu:      1536,
	})
	require.NoError(t, err)
	defer capture.Close()
	err = f.AddProcess("chrome.exe")
	// err = f.AddProcess("curl.exe")
	require.NoError(t, err)

	fmt.Println("prepared")
	for {
		s, err := capture.Capture(ctx)
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
