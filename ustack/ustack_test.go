package ustack_test

import (
	"context"
	"io"
	"math/rand"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"

	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Conn(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		seed  = time.Now().UnixNano()
		r     = rand.New(rand.NewSource(seed))
	)
	t.Log("seed", seed)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		st, err := ustack.NewUstack(link.NewList(16, 1536), saddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, s)

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept(context.Background())
		require.NoError(t, err)
		defer conn.Close()

		_, err = io.Copy(conn, conn)
		if errors.Is(err, gonet.ErrConnectReset) {
			return nil
		}
		return err
	})

	eg.Go(func() error {
		time.Sleep(time.Second)
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, c)

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r, conn, 0xffff)

		return nil
	})
	require.NoError(t, eg.Wait())
}

func Test_Conn_Client(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		seed  = int64(1709547794731834700) // time.Now().UnixNano()
		r     = rand.New(rand.NewSource(seed))
	)
	t.Log("seed", seed)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		st, err := ustack.NewUstack(link.NewList(16, 1536), saddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRawBy(t, st, s, caddr)

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept(context.Background())
		require.NoError(t, err)
		defer conn.Close()

		_, err = io.Copy(conn, conn)
		if errors.Is(err, gonet.ErrConnectReset) {
			return nil
		}
		return err
	})
	eg.Go(func() error {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, c)

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r, conn, 0xffff)
		return nil
	})
	require.NoError(t, eg.Wait())
}

func Test_Conn_Clients(t *testing.T) {
	var (
		caddr1 = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		caddr2 = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr  = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		seed   = time.Now().UnixNano()
		r1     = rand.New(rand.NewSource(seed))
		r2     = rand.New(rand.NewSource(seed))
	)
	t.Log("seed", seed)
	c1, s1 := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr1, saddr,
		test.ValidAddr, test.ValidChecksum,
	)
	c2, s2 := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr2, saddr,
		test.ValidAddr, test.ValidChecksum,
	)
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		st, err := ustack.NewUstack(link.NewList(16, 1536), saddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRawBy(t, st, s1, caddr1)
		UnicomStackAndRawBy(t, st, s2, caddr2)

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		for i := 0; i < 2; i++ {
			conn, err := l.Accept(context.Background())
			require.NoError(t, err)

			eg.Go(func() error {
				_, err = io.Copy(conn, conn)
				if errors.Is(err, gonet.ErrConnectReset) {
					return nil
				}
				return err
			})
		}
		return nil
	})

	// client1
	eg.Go(func() error {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr1.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, c1)

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr1, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r1, conn, 0xffff)
		return nil
	})
	// client2
	eg.Go(func() error {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr2.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, c2)

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr2, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r2, conn, 4086)
		return nil
	})
	require.NoError(t, eg.Wait())
}

func Test_Ustack_LinkEndpoint(t *testing.T) {
	// test ustack.LinkEndpoint().Close() will not close parent ustack
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
	)

	t.Run("std", func(t *testing.T) {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr.Addr())
		require.NoError(t, err)
		st = closePanic{st}

		lep, err := st.LinkEndpoint(123, netip.AddrPortFrom(netip.IPv4Unspecified(), 5678))
		require.NoError(t, err)
		require.NoError(t, lep.Close())
	})

	t.Run("pcap", func(t *testing.T) {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr.Addr())
		require.NoError(t, err)
		st = ustack.MustWrapPcap(st, filepath.Join(os.TempDir(), "test.pcap"))
		st = closePanic{st}

		lep, err := st.LinkEndpoint(123, netip.AddrPortFrom(netip.IPv4Unspecified(), 5678))
		require.NoError(t, err)
		require.NoError(t, lep.Close())
	})

}

type closePanic struct {
	ustack.Ustack
}

func (closePanic) Close() error { panic("") }
