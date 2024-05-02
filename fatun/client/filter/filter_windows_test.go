package filter

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_newAddrSyn(t *testing.T) {
	var (
		addr = netip.AddrPortFrom(netip.IPv4Unspecified(), 1)
	)

	as := newAddrSyn(time.Second * 4)
	for i := 0; i < 3; i++ {
		require.Equal(t, uint8(i+1), as.Upgrade(addr))
		time.Sleep(time.Second)
	}
	time.Sleep(time.Second * 2)
	require.Equal(t, uint8(1), as.Upgrade(addr))
}
