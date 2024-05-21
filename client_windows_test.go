//go:build windows
// +build windows

package fatun_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/peer"
	"github.com/lysShub/fatun/wrapper/pcap"
	"github.com/stretchr/testify/require"
)

func TestXxx(t *testing.T) {
	divert.MustLoad(divert.DLL)
	defer divert.Release()

	conn, err := fatcp.Dial[peer.Default]("8.137.91.200:443", &fatcp.Config{})
	// conn, err := fatcp.Dial[peer.Default]("103.94.185.61:443", &fatcp.Config{})
	require.NoError(t, err)
	defer conn.Close()
	fmt.Println("connect", conn.LocalAddr(), conn.RemoteAddr())

	c, err := fatun.NewClient[peer.Default](func(c *fatun.Client) { c.Conn = conn })
	require.NoError(t, err)

	filter, ok := c.Capturer.(interface{ Enable(process string) })
	require.True(t, ok)

	c.Capturer = pcap.WrapCapture(c.Capturer, "capture.pcap")

	err = c.Run()
	require.NoError(t, err)

	filter.Enable("aces.exe")

	time.Sleep(time.Hour * 3)
}
