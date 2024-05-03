package sconn

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func Test_heap(t *testing.T) {
	var puts = func(h *heap, n int) {
		i := 0
		for ; i < min(n, heapsize); i++ {
			pkt := packet.Make().Append([]byte{byte(i)})
			ok := h.put(pkt)
			if !ok {
				println()
			}
			require.True(t, ok)
		}

		for j := i; j < n; j++ {
			ok := h.put(packet.Make().Append([]byte{byte(j)}))
			require.False(t, ok)
		}
	}
	var pops = func(h *heap, n int) {
		var size atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < n+0xf; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				pkb := packet.Make()
				if h.pop(pkb) {
					size.Add(1)

					v := pkb.Bytes()

					require.Less(t, v[0], byte(n))
				}
			}()
		}

		wg.Wait()
		require.Equal(t, min(n, heapsize), int(size.Load()))
	}

	for n := 0; n < heapsize+8; n++ {
		require.Less(t, n, 0xff)

		h := &heap{}
		puts(h, n)
		pops(h, n)
	}
}
