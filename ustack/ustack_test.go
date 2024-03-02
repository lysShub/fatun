package ustack

import (
	"context"
	"io"
	"math/rand"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/ustack/link/channel"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Ustack(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())

		seed = time.Now().UnixNano()
		r    = rand.New(rand.NewSource(seed))
	)
	t.Log("seed", seed)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)

	// server
	srvRet := make(chan struct{})
	go func() {
		ctx := cctx.WithContext(context.Background())
		link := channel.New(4, 1536, "")
		ss, err := NewUstack(link, saddr, caddr)
		require.NoError(t, err)

		go func() {
			p := relraw.NewPacket(0, 1536)
			for {
				p.Sets(0, 1536)

				err := s.ReadCtx(ctx, p)
				require.NoError(t, err)

				err = ss.Inbound(p)
				require.NoError(t, err)
			}
		}()
		go func() {
			p := relraw.NewPacket(0, 1536)
			for {
				p.Sets(0, 1536)
				err := ss.Outbound(ctx, p)
				require.NoError(t, err)

				_, err = s.Write(p.Data())
				require.NoError(t, err)
			}
		}()

		conn := ss.Accept(ctx, time.Second)
		require.NoError(t, ctx.Err())

		_, err = io.Copy(conn, conn)
		require.NoError(t, err)
		close(srvRet)
	}()

	{ // client

		ctx := cctx.WithContext(context.Background())
		link := channel.New(4, 1536, "")
		cs, err := NewUstack(link, caddr, saddr)
		require.NoError(t, err)

		go func() {
			p := relraw.NewPacket(0, 1536)
			for {
				p.Sets(0, 1536)
				err := c.ReadCtx(ctx, p)
				require.NoError(t, err)

				err = cs.Inbound(p)
				require.NoError(t, err)
			}
		}()
		go func() {
			p := relraw.NewPacket(0, 1536)
			for {
				p.Sets(0, 1536)
				err := cs.Outbound(ctx, p)
				require.NoError(t, err)

				_, err = c.Write(p.Data())
				require.NoError(t, err)
			}
		}()

		conn := cs.Connect(ctx, time.Second)
		require.NoError(t, ctx.Err())

		for i := 0; i < 64; i++ {
			var msg = make([]byte, r.Int31()%1024+1)
			r.Read(msg)

			_, err = conn.Write(msg)
			require.NoError(t, err)

			var b = make([]byte, len(msg))
			_, err = io.ReadFull(conn, b)
			require.NoError(t, err)

			require.Equal(t, string(msg), string(b), i)
		}

		err = conn.Close()
		require.NoError(t, err)

		select {
		case <-srvRet:
		case <-time.After(time.Second * 3):
			t.Log("server return timeout")
			t.FailNow()
		}
	}
}
