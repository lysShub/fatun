package client_test

import (
	"context"
	proxy "itun/client"
	"net"
	"testing"
	"time"

	"github.com/lysShub/go-divert"
	"github.com/stretchr/testify/require"
)

var _ = divert.SetPath(`D:\OneDrive\code\go\go-divert\WinDivert.dll`)

var saddr = &net.UDPAddr{IP: net.ParseIP("192.168.21.146"), Port: 19986}

func TestProxy(t *testing.T) {
	pxyConn, err := net.DialUDP("udp", nil, saddr)
	require.NoError(t, err)

	pxy := proxy.ListenAndProxy(context.Background(), pxyConn, &proxy.Config{})

	// telnet 142.251.43.14 80

	var r = "tcp and remoteAddr=142.251.43.14 and remotePort=80"

	err = pxy.AddRule(r)
	require.NoError(t, err)

	t.Log("wait:")
	time.Sleep(time.Minute)
}

func TestRecv(t *testing.T) {
	var f = `ipv6 and tcp`

	h, err := divert.Open(f, divert.LAYER_NETWORK, 1, divert.FLAG_READ_ONLY)
	require.NoError(t, err)

	var b = make([]byte, 1536)
	var n int
	var addr divert.Address
	for {
		n, addr, err = h.Recv(b)
		require.NoError(t, err)
		t.Log("read:", n, addr.IPv6())
		return
	}

}
