package server_test

import (
	"testing"

	"github.com/lysShub/fatun/fatun/server"
	"github.com/stretchr/testify/require"
)

// func Test_TTLMap(t *testing.T) {
// 	var (
// 		caddr1 = netip.AddrPortFrom(test.RandIP(), test.RandPort())
// 		caddr2 = netip.AddrPortFrom(test.RandIP(), test.RandPort())

// 		saddr1 = netip.AddrPortFrom(test.RandIP(), test.RandPort())
// 		saddr2 = netip.AddrPortFrom(test.RandIP(), test.RandPort())
// 		saddr3 = netip.AddrPortFrom(test.RandIP(), test.RandPort())
// 		saddr3 = netip.AddrPortFrom(test.RandIP(), test.RandPort())
// 	)

// 	m := server.NewTTLMap(time.Second*10, test.LocIP())

// }

func Test_Port(t *testing.T) {
	{
		p := server.NewPort(0x12ab)
		require.True(t, p.Idle())
	}

	{
		p := server.NewPort(0x12ab)
		require.Equal(t, uint16(0x12ab), p.Port())
		require.False(t, p.Idle())
		require.True(t, p.Idle())
	}
}
