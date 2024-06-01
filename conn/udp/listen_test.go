package udp_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lysShub/fatun/conn/udp"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Test_Listen_Base(t *testing.T) {
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

	// echo server
	eg.Go(func() error {
		l, err := udp.Listen(saddr, maxSize)
		require.NoError(t, err)
		defer l.Close()

		for i := 0; i < len(caddrs); i++ {
			conn, err := l.Accept()
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			require.NoError(t, err)

			eg.Go(func() error {
				defer conn.Close()
				var b = make([]byte, 1536)

				for i := 0; i < 3; i++ {
					n, err := conn.Read(b)
					require.NoError(t, err)

					_, err = conn.Write(b[:n])
					require.NoError(t, err)
				}
				return nil
			})
		}
		return l.Close()
	})

	time.Sleep(time.Second)
	for _, e := range caddrs {
		caddr := e
		eg.Go(func() error {
			conn, err := net.DialUDP("udp", caddr, saddr)
			require.NoError(t, err)

			for i := 0; i < 3; i++ {
				msg := time.Now().String()
				_, err = conn.Write([]byte(msg))
				require.NoError(t, err)

				var b = make([]byte, 65)
				n, err := conn.Read(b)
				require.NoError(t, err)

				require.Equal(t, msg, string(b[:n]))
			}
			return nil
		})
	}
	eg.Wait()
}

func Test_Listen_Closed(t *testing.T) {

	t.Skip("todo:")

}
