package proxy_test

import (
	"context"
	"itun/proxy"
	"net"
	"testing"

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

	var r = "!loopback and tcp and remoteAddr=142.251.43.14 and remotePort=80"

	err = pxy.AddRule(r)
	require.NoError(t, err)
}
