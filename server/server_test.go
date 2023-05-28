package server_test

import (
	"itun/server"
	"net"
	"testing"

	"github.com/lysShub/accept/udp"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {

	l, err := udp.NewListener(&net.UDPAddr{Port: 19986}, 1532)
	require.NoError(t, err)

	err = server.ListenAndServe(l)
	require.NoError(t, err)

	// conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 19986})
	// require.NoError(t, err)

	// var b = make([]byte, 1536)
	// for {
	// 	n, err := conn.Read(b)
	// 	require.NoError(t, err)
	// 	fmt.Println(n)
	// }

}
