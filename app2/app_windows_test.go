package app2_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	gdivert "github.com/lysShub/divert-go"
	"github.com/lysShub/itun/app2/client"
	"github.com/lysShub/itun/app2/client/capture"
	"github.com/lysShub/itun/app2/client/filter"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/relraw/tcp/divert"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	gdivert.Load(gdivert.Mem)
	defer gdivert.Release()
	ctx := context.Background()

	f := filter.NewMock("curl.exe")
	capture, err := capture.NewCapture(cctx.WithContext(ctx), f)
	require.NoError(t, err)

	var c *client.Client
	if true {
		cfg := &client.Config{
			Config: config.Config{
				PrevPackets:      pps,
				HandShakeTimeout: time.Second * 5,
				SwapKey:          &crypto.TokenClient{Tokener: &tkClient{}},
			},
			MTU: 1536,
		}

		raw, err := divert.Connect(caddr, saddr)
		require.NoError(t, err)

		c, err = client.NewClient(ctx, raw, capture, cfg)
		require.NoError(t, err)
		defer c.Close(nil)

		fmt.Println("connected")
	}

	fmt.Println("prepared")
	for {
		s, err := capture.Get(ctx)
		require.NoError(t, err)

		fmt.Println("Capture Session", s.String())

		// capture.Del(s.Session())
		// return

		err = c.AddProxy(s)
		require.NoError(t, err)

		fmt.Println("AddProxy", s.String())

	}
}

func Test_Capture(t *testing.T) {
	gdivert.Load(gdivert.Mem)
	defer gdivert.Release()

	ctx := cctx.WithContext(context.Background())

	f := filter.NewMock("curl.exe")

	c, err := capture.NewCapture(ctx, f)
	require.NoError(t, err)

	for {
		s, err := c.Get(context.Background())
		require.NoError(t, err)

		fmt.Println(s.String())

		c.Del(s.Session())

		return
	}

}

func Test_TCP(t *testing.T) {
	conn, err := net.DialTCP("tcp", test.TCPAddr(caddr), test.TCPAddr(saddr))
	require.NoError(t, err)

	_, err = conn.Write([]byte("hello"))
	require.NoError(t, err)

	_, err = conn.Write([]byte("world"))
	require.NoError(t, err)

	var b = make([]byte, 10)

	_, err = io.ReadFull(conn, b)
	require.NoError(t, err)

	fmt.Println(len(b))
}
