package udp_test

import (
	"context"
	"io"
	"math/rand"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/udp"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_UDP_Builtin(t *testing.T) {
	var (
		saddr  = netip.AddrPortFrom(test.LocIP(), 8080)
		server = &udp.Config{
			MaxRecvBuff:     1500,
			PcapBuiltinPath: "builtin-server.pcap",
		}
		client = &udp.Config{
			MaxRecvBuff:     1500,
			PcapBuiltinPath: "builtin-client.pcap",
		}
	)
	os.Remove(server.PcapBuiltinPath)
	os.Remove(client.PcapBuiltinPath)

	eg, ctx := errgroup.WithContext(context.Background())

	// echo server
	eg.Go(func() error {
		l, err := udp.Listen[conn.Default](saddr.String(), server)
		require.NoError(t, err)
		defer l.Close()

		c, err := l.Accept()
		require.NoError(t, err)
		defer c.Close()

		eg.Go(func() error {
			var p = conn.NewDefaultPeer()
			var b = packet.Make(server.MaxRecvBuff)
			err := c.Recv(p, b.Sets(64, 0xffff))
			return err
		})

		tcp, err := c.BuiltinConn(ctx)
		require.NoError(t, err)

		_, err = io.Copy(tcp, tcp)
		require.NoError(t, err)
		return l.Close()
	})

	// client
	eg.Go(func() error {
		time.Sleep(time.Second)

		c, err := udp.Dial[conn.Default](saddr.String(), client)
		require.NoError(t, err)
		defer c.Close()

		eg.Go(func() error {
			var p = conn.NewDefaultPeer()
			var b = packet.Make(client.MaxRecvBuff)
			err := c.Recv(p, b.Sets(64, 0xffff))
			return err
		})

		tcp, err := c.BuiltinConn(ctx)
		require.NoError(t, err)

		test.ValidPingPongConn(t, rand.New(rand.NewSource(0)), tcp, 0xffff)
		return nil
	})

	eg.Wait()
}

func Test_UDP_Packets(t *testing.T) {
	var (
		saddr   = netip.AddrPortFrom(test.LocIP(), 8080)
		packets = 0xff
		config  = &udp.Config{
			MaxRecvBuff: 1500,
		}
	)

	eg, _ := errgroup.WithContext(context.Background())

	// echo server
	eg.Go(func() error {
		l, err := udp.Listen[conn.Default](saddr.String(), config)
		require.NoError(t, err)
		defer l.Close()

		c, err := l.Accept()
		require.NoError(t, err)
		defer c.Close()

		var p = conn.NewDefaultPeer()
		var b = packet.Make(config.MaxRecvBuff)
		for i := 0; i < packets; i++ {
			err := c.Recv(p, b.Sets(64, 0xffff))
			require.NoError(t, err)

			err = c.Send(p, b)
			require.NoError(t, err)
		}
		return nil
	})

	// client
	eg.Go(func() error {
		time.Sleep(time.Second)

		c, err := udp.Dial[conn.Default](saddr.String(), config)
		require.NoError(t, err)
		defer c.Close()

		var p = conn.NewDefaultPeer()
		var b = packet.Make(config.MaxRecvBuff)
		for i := 0; i < packets; i++ {
			msg := time.Now().String()
			p.Reset(header.TCPProtocolNumber, test.RandIP())

			err = c.Send(p, b.Sets(64, 0).Append([]byte(msg)))
			require.NoError(t, err)

			err = c.Recv(p, b.Sets(64, 0xffff))
			require.NoError(t, err)

			require.Equal(t, msg, string(b.Bytes()))
		}
		return nil

	})

	eg.Wait()

}

/*

	eg.Go(func() error {
		var p = conn.NewDefaultPeer()
		var b = packet.Make(config.MaxRecvBuff)

		for i := 0; i < packets; i++ {
			err = c.Recv(p, b.Sets(0, 0xffff))
			require.NoError(t, err)

			err = c.Send(p, b)
			require.NoError(t, err)
		}
		return nil
	})


			eg.Go(func() error {
			var p = conn.NewDefaultPeer()
			var b = packet.Make(config.MaxRecvBuff)
			for i := 0; i < 0xff; i++ {
				p.Reset(header.TCPProtocolNumber, test.RandIP())

				msg := time.Now().String()
				err = c.Send(p, b.Sets(64, 0).Append([]byte(msg)))
				require.NoError(t, err)

				err = c.Recv(p, b.Sets(0, 0xffff))
				require.NoError(t, err)

				require.Equal(t, msg, string(b.Bytes()))
			}
			return nil
		})
*/
