package rule_test

import (
	"fmt"
	"itun/proxy/rule"
	"testing"

	"github.com/lysShub/go-divert"
	"github.com/stretchr/testify/require"
)

var _ = divert.SetPath(`D:\OneDrive\code\go\go-divert\WinDivert.dll`)

func TestXxx(t *testing.T) {
	rs := rule.NewRules()
	err := rs.AddBuiltinRule()
	require.NoError(t, err)

	pch := rs.Proxyer()
	for f := range pch {
		fmt.Println(f)
	}
}

func TestSocket(t *testing.T) {

	var f = "!loopback and tcp and remoteAddr=142.251.43.14 and remotePort=80"

	h, err := divert.Open(f, divert.LAYER_SOCKET, 0, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF) // |divert.FLAG_SNIFF
	require.NoError(t, err)

	// and remoteAddr=142.251.43.14 and remotePort=80
	// and remoteAddr=39.156.66.10 and remotePort=80

	for {
		_, addr, err := h.Recv(nil)
		require.NoError(t, err)

		f := addr.Socket()

		a := f.RemoteAddr().String()

		fmt.Print(a)
	}
}

func TestNetwork(t *testing.T) {

	var f = "!loopback and ifIdx=58 and remoteAddr=142.251.43.14 and remotePort=80"

	hdl, err := divert.Open(f, divert.LAYER_NETWORK_FORWARD, 0, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	require.NoError(t, err)

	var b = make([]byte, 1536)
	for {
		_, addr, err := hdl.Recv(b)
		require.NoError(t, err)
		n := addr.Network()
		println(n)
	}
}

func TestCompile(t *testing.T) {

	// var f = "!loopback and ifIdx=58 and remoteAddr=142.251.43.14 and remotePort=80"
	var f = "remoteAddr=142.251.43.14 and remotePort=80"
	// "@WinDiv_WY_1+WW27FMAOk1VV=WWLXX_2WWW2mAX"
	// "@WinDiv_WY_1+WW27FMAOk1VV=WWLXX_2WWW2nAX"

	s, err := divert.HelperCompileFilter(f, divert.LAYER_NETWORK)

	t.Log(s, err)
}
