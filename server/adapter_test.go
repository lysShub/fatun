package server_test

import (
	"net/netip"
	"testing"

	"github.com/lysShub/itun/protocol"
	"github.com/lysShub/itun/server"
	"github.com/lysShub/relraw"
	"github.com/stretchr/testify/require"
)

var (
	addr1 = netip.AddrPortFrom(netip.AddrFrom4([4]byte{8, 8, 8, 8}), 80)
	addr2 = netip.AddrPortFrom(netip.AddrFrom4([4]byte{8, 8, 8, 8}), 8080)
	addr3 = netip.AddrPortFrom(netip.AddrFrom4([4]byte{8, 8, 8, 9}), 80)
)

func Test_AddrSet(t *testing.T) {
	var as = &server.AddrSet{}

	as.Add(addr1)
	as.Add(addr1)
	require.Zero(t, as.Find(addr1))
	require.True(t, as.Has(addr1))

	as.Add(addr2)
	require.True(t, as.Has(addr2))

	as.Del(addr1)
	require.False(t, as.Has(addr1))
	require.Equal(t, 1, as.Len())
}

func Test_Port_Adapter(t *testing.T) {

	t.Run("reuse", func(t *testing.T) {
		ap := server.NewPortAdapter(relraw.LocalAddr())
		defer func() {
			require.NoError(t, ap.Close())
		}()

		p1, err := ap.GetPort(protocol.TCP, addr1)
		require.NoError(t, err)

		p2, err := ap.GetPort(protocol.TCP, addr2)
		require.NoError(t, err)

		require.Equal(t, p1, p2)

		p3, err := ap.GetPort(protocol.TCP, addr3)
		require.NoError(t, err)

		require.Equal(t, p2, p3)

	})

	t.Run("repead-get", func(t *testing.T) {
		ap := server.NewPortAdapter(relraw.LocalAddr())
		defer func() {
			require.NoError(t, ap.Close())
		}()

		p1, err := ap.GetPort(protocol.TCP, addr1)
		require.NoError(t, err)

		p2, err := ap.GetPort(protocol.TCP, addr1)
		require.NoError(t, err)

		require.NotEqual(t, p1, p2)
	})

	t.Run("add-invalid-proto", func(t *testing.T) {
		ap := server.NewPortAdapter(relraw.LocalAddr())
		defer func() {
			require.NoError(t, ap.Close())
		}()

		p1, err := ap.GetPort(123, addr1)
		require.Equal(t, protocol.ErrInvalidProto(123), err)
		require.Zero(t, p1)
	})

	t.Run("add-invalid-addr", func(t *testing.T) {
		ap := server.NewPortAdapter(relraw.LocalAddr())
		defer func() {
			require.NoError(t, ap.Close())
		}()

		p1, err := ap.GetPort(protocol.TCP, netip.AddrPort{})
		require.Equal(t, protocol.ErrInvalidAddr(netip.Addr{}), err)
		require.Zero(t, p1)
	})
}
