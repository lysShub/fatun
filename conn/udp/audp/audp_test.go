package audp_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/lysShub/fatun/conn/udp/audp"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Test_Audp(t *testing.T) {
	var (
		maxSize = 1500
		saddr   = &net.UDPAddr{IP: test.LocIP().AsSlice(), Port: 8080}

		caddrs = []*net.UDPAddr{
			{IP: test.LocIP().AsSlice(), Port: 1234},
			{IP: test.LocIP().AsSlice(), Port: 1235},
			{IP: test.LocIP().AsSlice(), Port: 1236},
		}
	)
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		l, err := audp.Listen(saddr, maxSize)
		require.NoError(t, err)
		defer l.Close()

		gg, _ := errgroup.WithContext(context.Background())
		for i := 0; i < len(caddrs); i++ {
			conn, err := l.Accept()
			require.NoError(t, err)

			gg.Go(func() error {
				defer conn.Close()
				var b = make([]byte, 1536)
				n, err := conn.Read(b)
				require.NoError(t, err)

				_, err = conn.Write(b[:n])
				require.NoError(t, err)
				return nil
			})
		}
		gg.Wait()
		return nil
	})

	time.Sleep(time.Second)
	msg := "hello world"
	for _, e := range caddrs {
		caddr := e
		eg.Go(func() error {
			conn, err := net.DialUDP("udp", caddr, saddr)
			require.NoError(t, err)

			_, err = conn.Write([]byte(msg))
			require.NoError(t, err)

			var b = make([]byte, 16)
			n, err := conn.Read(b)
			require.NoError(t, err)

			require.Equal(t, msg, string(b[:n]))
			return nil
		})
	}
	eg.Wait()
}
