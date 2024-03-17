//go:build windows
// +build windows

package app_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	gdivert "github.com/lysShub/divert-go"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/app/client/capture"
	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/relraw/tcp/divert"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	gdivert.Load(gdivert.Mem)
	defer gdivert.Release()
	ctx := context.Background()

	f := filter.NewMock("curl.exe")
	capture, err := capture.NewCapture(f)
	require.NoError(t, err)
	defer capture.Close()

	var c *client.Client
	if true {
		cfg := &app.Config{
			Config: sconn.Config{
				PrevPackets: pps,
				SwapKey:     &crypto.TokenClient{Tokener: &tkClient{}},
				MTU:         1536,
			},
			Logger: slog.NewJSONHandler(os.Stdout, nil),
		}

		raw, err := divert.Connect(caddr, saddr)
		require.NoError(t, err)
		conn, err := sconn.Dial(raw, &cfg.Config)
		require.NoError(t, err)

		c, err = client.NewClient(ctx, conn, cfg)
		require.NoError(t, err)
		defer c.Close()

		fmt.Println("connected")
	}

	fmt.Println("prepared")
	for {
		s, err := capture.Get(ctx)
		require.NoError(t, err)

		fmt.Println("Capture Session", s.String())

		// capture.Del(s.Session())
		// return

		err = c.AddSession(ctx, s)
		require.NoError(t, err)

		fmt.Println("AddProxy", s.String())

	}
}
