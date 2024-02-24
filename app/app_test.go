package app

import (
	"context"
	"fmt"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lysShub/itun/app/client"
	"github.com/lysShub/itun/app/server"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestXxx(t *testing.T) {
	t.Skip("todo")

	var (
		caddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, 0, 1}), 19986)
		saddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{1, 1, 1, 1}), 8080)
	)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)

	// server
	go func() {
		l := &listenerWrap{conn: s}
		cfg := &server.Config{
			Sconn: sconn.Server{
				BaseConfig: sconn.BaseConfig{
					PrevPackets: pps,
				},
				SwapKey: &sconn.TokenServer{Valider: &tkServer{}},
			},
			MTU: 1536,
		}

		server.ListenAndServe(context.Background(), l, cfg)
	}()

	{ // client
		cfg := &client.Config{
			Sconn: sconn.Client{
				BaseConfig: sconn.BaseConfig{
					PrevPackets: pps,
				},
				SwapKey: &sconn.TokenClient{Tokener: &tkClient{}},
			},
			MTU: 1536,
		}

		ctx := context.Background()

		ct, err := client.NewClient(ctx, c, cfg)
		require.NoError(t, err)

		ct.Close()

		// err = ct.AddProxy(itun.Session{})
		// require.NoError(t, err)
	}

}

type listenerWrap struct {
	conn     relraw.RawConn
	accepted atomic.Bool
}

var _ relraw.Listener = (*listenerWrap)(nil)

func (l *listenerWrap) Accept() (relraw.RawConn, error) {
	if l.accepted.CompareAndSwap(false, true) {
		return l.conn, nil
	} else {
		time.Sleep(time.Hour * 20)
		panic("")
	}
}

func (l *listenerWrap) Addr() netip.AddrPort {
	return l.conn.LocalAddrPort()
}
func (l *listenerWrap) Close() error {
	return nil
}

type tkClient struct{}

func (c *tkClient) Token() (tk []byte, key sconn.Key, err error) {
	return []byte("hello"), sconn.Key{1: 1}, nil
}

type tkServer struct{}

func (c *tkServer) Valid(tk []byte) (key sconn.Key, err error) {
	if string(tk) == "hello" {
		return sconn.Key{1: 1}, nil
	}
	return sconn.Key{}, fmt.Errorf("invalid token")
}

var pps = sconn.PrevPackets{
	header.TCP("hello"),
	header.TCP("world"),
}
