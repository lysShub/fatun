package server_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {

	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 19986})
	require.NoError(t, err)

	var b = make([]byte, 1536)
	for {
		n, err := conn.Read(b)
		require.NoError(t, err)
		fmt.Println(n)
	}

}
