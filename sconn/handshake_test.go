package sconn

import (
	"context"
	"math/rand"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lysShub/fatun/sconn/crypto"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_heap(t *testing.T) {
	var puts = func(h *heap, n int) {
		i := 0
		for ; i < min(n, heapsize); i++ {
			pkt := packet.Make().Append([]byte{byte(i)})
			ok := h.put(pkt)
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
		for i := 0; i < n+0xff; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				pkb := packet.From(make([]byte, 1))
				if h.pop(pkb) {
					size.Add(1)

					require.Equal(t, 1, pkb.Data())
					require.Less(t, pkb.Bytes()[0], byte(n))
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

func Test_Handshake_Context_Cancel(t *testing.T) {
	t.Run("PrevPackets-Exceeded", func(t *testing.T) {
		var (
			caddr = netip.AddrPortFrom(test.LocIP(), 19986)
			saddr = netip.AddrPortFrom(test.LocIP(), 8080)
			cfg   = &Config{
				// buil enough big pss
				PSS: func() PrevSegmets {
					var pss [][]byte
					for i := 0; i < 0xffff; i++ {
						pss = append(pss, make([]byte, rand.Int()%1023+1))
					}
					return pss
				}(),
				Key:             sign,
				MaxRecvBuffSize: 1536,
				MTU:             1500,
			}
		)
		require.NoError(t, cfg.Init())

		c, s := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.05), test.Delay(time.Millisecond*50),
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		eg, ctx := errgroup.WithContext(ctx)

		// server
		eg.Go(func() error {
			l, err := NewListener(test.NewMockListener(t, s), cfg)
			require.NoError(t, err)
			defer l.Close()

			conn, err := l.AcceptCtx(ctx)
			require.NoError(t, err)
			_, err = conn.TCP(ctx)
			require.True(t, errors.Is(err, context.DeadlineExceeded), err)
			return nil
		})

		// client
		eg.Go(func() error {
			_, err := DialCtx(ctx, c, cfg)
			require.True(t, errors.Is(err, context.DeadlineExceeded), err)
			return nil
		})

		eg.Wait()
	})

	t.Run("Swapke-Cancle", func(t *testing.T) {
		var (
			caddr = netip.AddrPortFrom(test.LocIP(), 19986)
			saddr = netip.AddrPortFrom(test.LocIP(), 8080)
			sign  = make([]byte, 1024*1024*8)
			cfg   = &Config{
				PSS: pss,
				Key: &Sign{
					Sign:   sign,
					Parser: func(context.Context, []byte) (crypto.Key, error) { return crypto.Key{1: 1}, nil },
				},
				MaxRecvBuffSize: 1536,
				MTU:             1500,
			}
		)
		rand.New(rand.NewSource(0)).Read(sign) // avoid gob compress
		c, s := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.1), test.Delay(time.Millisecond*50),
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			l, err := NewListener(test.NewMockListener(t, s), cfg)
			require.NoError(t, err)
			defer l.Close()

			conn, err := l.AcceptCtx(ctx)
			require.NoError(t, err)
			_, err = conn.TCP(ctx)
			require.True(t, errors.Is(err, context.Canceled), err)
			return nil
		})

		// client
		eg.Go(func() error {
			_, err := DialCtx(ctx, c, cfg)
			require.True(t, errors.Is(err, os.ErrDeadlineExceeded), err)
			return nil
		})

		time.Sleep(time.Second)
		cancel()
		eg.Wait()
	})
}
