package sconn_test

import (
	"context"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Handshake_Ctx(t *testing.T) {

	t.Run("PrevPackets ", func(t *testing.T) {
		var (
			caddr = netip.AddrPortFrom(test.LocIP(), 19986)
			saddr = netip.AddrPortFrom(test.LocIP(), 8080)
			cfg   = sconn.Config{
				PrevPackets: func() sconn.PrevPackets {
					var pps [][]byte
					for i := 0; i < 0xffff; i++ {
						pps = append(pps, make([]byte, rand.Int()%1023+1))
					}
					return pps
				}(),
				SwapKey: sign,
				MTU:     1500,
			}
		)
		c, s := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.05), test.Delay(time.Millisecond*50),
		)
		eg, _ := errgroup.WithContext(context.Background())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		eg.Go(func() error {
			l, err := sconn.NewListener(test.NewMockListener(t, s), &cfg)
			require.NoError(t, err)
			defer l.Close()

			_, err = l.AcceptCtx(ctx)
			require.True(t, errors.Is(err, io.EOF), err)
			return nil
		})

		// client
		eg.Go(func() error {
			_, err := sconn.DialCtx(ctx, c, &cfg)
			require.True(t, errors.Is(err, os.ErrDeadlineExceeded), err)
			return nil
		})

		time.Sleep(time.Second)
		cancel()
		eg.Wait()
	})

	t.Run("Swapkey", func(t *testing.T) {
		var (
			caddr = netip.AddrPortFrom(test.LocIP(), 19986)
			saddr = netip.AddrPortFrom(test.LocIP(), 8080)
			sign  = make([]byte, 1024*1024*8)
			cfg   = sconn.Config{
				PrevPackets: pps,
				SwapKey: &sconn.Sign{
					Sign:   sign,
					Parser: func(sign []byte) (crypto.Key, error) { return crypto.Key{1: 1}, nil },
				},
				MTU: 1500,
			}
		)
		rand.New(rand.NewSource(0)).Read(sign)
		c, s := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.1), test.Delay(time.Millisecond*50),
		)
		eg, _ := errgroup.WithContext(context.Background())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		eg.Go(func() error {
			l, err := sconn.NewListener(test.NewMockListener(t, s), &cfg)
			require.NoError(t, err)
			defer l.Close()

			_, err = l.AcceptCtx(ctx)
			require.True(t, errors.Is(err, context.Canceled), err)
			return nil
		})

		// client
		eg.Go(func() error {
			_, err := sconn.DialCtx(ctx, c, &cfg)
			require.True(t, errors.Is(err, os.ErrDeadlineExceeded), err)
			return nil
		})

		time.Sleep(time.Second)
		cancel()
		eg.Wait()
	})
}

var (
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

func Test_Ctr_Conn(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), 19986) // test.RandPort()
		saddr = netip.AddrPortFrom(test.LocIP(), 8080)  // test.RandPort()
		mtu   = 1500
		cfg   = sconn.Config{
			PrevPackets: pps,
			SwapKey:     sign,
			MTU:         mtu,
		}
	)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum, test.PacketLoss(0.01), test.Delay(time.Millisecond*50),
	)
	// wc, err := test.WrapPcap(c, `test.pcap`)
	// require.NoError(t, err)
	// defer wc.Close()

	eg, ctx := errgroup.WithContext(context.Background())

	// echo server
	eg.Go(func() error {
		l, err := sconn.NewListener(test.NewMockListener(t, s), &cfg)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		eg.Go(func() error {
			var p = packet.Make(0, mtu)
			_, err := conn.Recv(ctx, p)
			require.True(t, errors.Is(err, net.ErrClosed), err)
			return nil
		})

		tcp := conn.TCP()
		_, err = io.Copy(tcp, tcp)
		return err
	})

	// client
	eg.Go(func() error {
		conn, err := sconn.Dial(c, &cfg)
		require.NoError(t, err)
		defer conn.Close()

		eg.Go(func() error {
			var p = packet.Make(0, mtu)
			_, err := conn.Recv(ctx, p)
			require.True(t, errors.Is(err, net.ErrClosed), err)
			return nil
		})

		rander := rand.New(rand.NewSource(0))
		test.ValidPingPongConn(t, rander, conn.TCP(), 0xffff)

		return nil
	})

	err := eg.Wait()
	ok := errors.Is(err, gonet.ErrConnectReset) || err == nil
	require.True(t, ok, err)
}

func Test_Conn(t *testing.T) {
	t.Skip("todo")
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		sid   = session.ID(1234)
		mtu   = 1500
	)

	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum, //test.PacketLoss(0.1),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	// echo server
	eg.Go(func() error {
		var cfg = sconn.Config{
			PrevPackets: pps,
			SwapKey:     sign,
		}

		l, err := sconn.NewListener(test.NewMockListener(t, s), &cfg)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		eg.Go(func() error {
			var pkt = packet.Make(64, mtu)
			for {
				id, err := conn.Recv(ctx, pkt.SetHead(64))
				require.NoError(t, err)
				require.Equal(t, sid, id)
				err = conn.Send(ctx, pkt, sid)
				require.NoError(t, err)
			}
		})

		tcp := conn.TCP()
		io.Copy(tcp, tcp)
		return nil
	})

	// client
	eg.Go(func() error {
		var cfg = sconn.Config{
			PrevPackets: pps,
			SwapKey:     sign,
		}
		wc, err := test.WrapPcap(c, "./test.pcap")
		require.NoError(t, err)
		defer wc.Close()

		conn, err := sconn.Dial(wc, &cfg)
		require.NoError(t, err)
		defer conn.Close()

		rander := rand.New(rand.NewSource(0))
		eg.Go(func() error {
			var pkt = packet.Make(64, mtu)
			for {
				rander.Read(pkt.SetData(rand.Int() % mtu).Bytes())
				err := conn.Send(ctx, pkt, sid)
				require.NoError(t, err)

				time.Sleep(time.Millisecond * 10)
			}
		})
		eg.Go(func() error {
			var p = packet.Make(64, mtu)
			for {
				id, err := conn.Recv(ctx, p.SetHead(64))
				require.NoError(t, err)
				require.Equal(t, sid, id)
			}
		})

		test.ValidPingPongConn(t, rander, conn.TCP(), 0xff)

		cancel()
		return nil
	})

	eg.Wait()
}
