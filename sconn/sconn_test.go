package sconn_test

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/netip"
	"os"
	"testing"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
	"golang.org/x/sync/errgroup"

	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Ctr_Conn(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), 19986) // test.RandPort()
		saddr = netip.AddrPortFrom(test.LocIP(), 8080)  // test.RandPort()
		mtu   = 1500

		sign = &sconn.Sign{
			Sign: []byte("0123456789abcdef"),
			Parser: func(sign []byte) (crypto.Key, error) {
				if string(sign) == "0123456789abcdef" {
					return crypto.Key{9: 1}, nil
				}
				return crypto.Key{}, errors.New("invalid sign")
			},
		}
		pps = sconn.PrevPackets{
			header.TCP("hello"),
			header.TCP("world"),
			header.TCP("abcdef"),
			header.TCP("xyz"),
		}
	)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.1),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	// echo server
	eg.Go(func() error {
		var cfg = sconn.Config{
			PrevPackets:  pps,
			SwapKey:      sign,
			HandshakeMTU: mtu,
		}

		l, err := sconn.NewListener(test.NewMockListener(t, s), &cfg)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		eg.Go(func() error {
			var p = packet.Make(0, mtu)
			_, err := conn.Recv(ctx, p)
			require.True(t, errors.Is(err, os.ErrClosed), err)
			return nil
		})

		io.Copy(conn, conn)
		return nil
	})

	// client
	eg.Go(func() error {
		var cfg = sconn.Config{
			PrevPackets:  pps,
			SwapKey:      sign,
			HandshakeMTU: mtu,
		}
		// wc, err := test.WrapPcap(c, "./test.pcap")
		// require.NoError(t, err)
		// defer wc.Close()

		conn, err := sconn.Dial(c, &cfg)
		require.NoError(t, err)
		defer conn.Close()

		eg.Go(func() error {
			var p = packet.Make(0, mtu)
			_, err := conn.Recv(ctx, p)
			require.True(t, errors.Is(err, os.ErrClosed), err)
			return nil
		})

		rander := rand.New(rand.NewSource(0))
		test.ValidPingPongConn(t, rander, conn, 0xffff)

		return nil
	})

	eg.Wait()
}
