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

func Test_Heap(t *testing.T) {
	t.Run("empt", func(t *testing.T) {
		var h = NewHeap[int]()

		require.Zero(t, h.Peek())
		require.Zero(t, h.Pop())
		h.Put(1)
		require.Equal(t, 1, h.Peek())
		require.Equal(t, 1, h.Pop())
		require.Zero(t, h.Peek())
	})

	t.Run("grow", func(t *testing.T) {
		var h = NewHeap[int]()
		for i := 0; i < initHeapCap; i++ {
			h.Put(i)
		}

		h.Put(1234)

		for i := 0; i < initHeapCap; i++ {
			require.Equal(t, i, h.Pop())
		}
		require.Equal(t, 1234, h.Pop())
	})
}
