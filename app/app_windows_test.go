//go:build windows
// +build windows

package app_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	_ "net/http/pprof"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/sockit/conn/tcp"
	sd "github.com/lysShub/sockit/conn/tcp/divert"
	"github.com/lysShub/sockit/packet"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	divert.Load(divert.DLL)
	defer divert.Release()
	ctx := context.Background()
	fmt.Println("启动")

	f := filter.NewMock("chrome.exe")
	// f := filter.NewMock("curl.exe")
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
		// wraw, err := test.WrapPcap(raw, "raw.pcap")
		// require.NoError(t, err)
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
