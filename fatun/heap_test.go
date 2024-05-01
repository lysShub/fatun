package fatun_test

import (
	"testing"

	"github.com/lysShub/fatun/fatun"
	"github.com/stretchr/testify/require"
)

func Test_Heap(t *testing.T) {

	const initHeapCap = 64

	t.Run("empt", func(t *testing.T) {
		var h = fatun.NewHeap[int](initHeapCap)

		require.Zero(t, h.Peek())
		require.Zero(t, h.Pop())
		h.Put(1)
		require.Equal(t, 1, h.Peek())
		require.Equal(t, 1, h.Pop())
		require.Zero(t, h.Peek())
	})

	t.Run("grow", func(t *testing.T) {
		var h = fatun.NewHeap[int](initHeapCap)
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
