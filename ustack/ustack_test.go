package ustack_test

import (
	"context"
	"io"
	"math/rand"
	"net/netip"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/test"
	"github.com/lysShub/rsocket/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
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

	var rets = make(chan string, 1)

	go func() {
		st, err := ustack.NewUstack(link.NewList(16, 1536), saddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(s, 1536))

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept(context.Background())
		require.NoError(t, err)
		defer conn.Close()

		_, err = io.Copy(conn, conn)
		if !IsGvisorClose(err) {
			require.NoError(t, err)
		}
		rets <- "server"
	}()

	go func() { // client
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(c, 1536))

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r, conn, 0xffff)
	}()

	<-rets
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

	var rets = make(chan string, 1)

	go func() { // server
		st, err := ustack.NewUstack(link.NewList(16, 1536), saddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRawBy(t, st, itun.WrapRawConn(s, 1536), caddr)

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept(context.Background())
		require.NoError(t, err)
		defer conn.Close()

		_, err = io.Copy(conn, conn)
		if !IsGvisorClose(err) {
			require.NoError(t, err)
		}
		rets <- "server"
	}()

	go func() {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(c, 1536))

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r, conn, 0xffff)
	}()

	<-rets
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

	// server
	go func() {
		st, err := ustack.NewUstack(link.NewList(16, 1536), saddr.Addr())
		require.NoError(t, err)
		UnicomStackAndRawBy(t, st, itun.WrapRawConn(s1, 1536), caddr1)
		UnicomStackAndRawBy(t, st, itun.WrapRawConn(s2, 1536), caddr2)

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		for {
			conn, err := l.Accept(context.Background())
			require.NoError(t, err)

			go func() {
				_, err = io.Copy(conn, conn)
				if !IsGvisorClose(err) {
					require.NoError(t, err)
				}
			}()
		}
	}()

	var rets = make(chan string, 2)

	// client 1
	go func() {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr1.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(c1, 1536))

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr1, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r1, conn, 0xffff)
		rets <- "client1"
	}()

	// client 2
	go func() {
		st, err := ustack.NewUstack(link.NewList(16, 1536), caddr2.Addr())
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(c2, 1536))

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr2, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r2, conn, 4086)
		rets <- "client2"
	}()

	t.Log(<-rets, "retrun")
	t.Log(<-rets, "retrun")
}

func UnicomStackAndRaw(t *testing.T, s ustack.Ustack, raw *itun.RawConn) {
	go func() {
		mtu := raw.MTU()
		var p = rsocket.NewPacket(0, mtu)

		for {
			p.Sets(0, mtu)
			s.Outbound(context.Background(), p)
			if p.Len() == 0 {
				return
			}

			// fmt.Println("inbound")

			err := raw.Write(context.Background(), p)
			require.NoError(t, err)

			if debug.Debug() {
				p.SetHead(0)
				test.ValidIP(t, p.Data())
			}
		}
	}()
	go func() {
		mtu := raw.MTU()
		var p = rsocket.NewPacket(0, mtu)

		for {
			p.Sets(0, mtu)
			err := raw.Read(context.Background(), p)
			if errors.Is(err, io.EOF) {
				return
			}
			require.NoError(t, err)

			// fmt.Println("outbound")

			p.SetHead(0)
			test.ValidIP(t, p.Data())

			s.Inbound(p)
		}
	}()
}

func UnicomStackAndRawBy(t *testing.T, s ustack.Ustack, raw *itun.RawConn, dst netip.AddrPort) {
	go func() {
		mtu := raw.MTU()
		var p = rsocket.NewPacket(0, mtu)

		for {
			p.Sets(0, mtu)
			s.OutboundBy(context.Background(), dst, p)
			if p.Len() == 0 {
				return
			}

			err := raw.Write(context.Background(), p)
			require.NoError(t, err)

			if debug.Debug() {
				p.SetHead(0)
				test.ValidIP(t, p.Data())
			}
		}
	}()
	go func() {
		mtu := raw.MTU()
		var p = rsocket.NewPacket(0, mtu)

		for {
			p.Sets(0, mtu)
			err := raw.Read(context.Background(), p)
			if errors.Is(err, io.EOF) {
				return
			}
			require.NoError(t, err)

			p.SetHead(0)
			test.ValidIP(t, p.Data())

			s.Inbound(p)
		}
	}()
}

func Base(err error) error {
	e := errors.Unwrap(err)
	if e == nil {
		return err
	}
	return Base(e)
}

func IsGvisorClose(err error) bool {
	if err == nil {
		return false
	}
	err = Base(err)

	return err.Error() == (&tcpip.ErrConnectionReset{}).String()
}
