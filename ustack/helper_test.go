package ustack

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/netip"
	"runtime"
	"testing"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Ustack_TCP(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP, test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP, test.RandPort())

		seed int64 = time.Now().UnixNano()
		r          = rand.New(rand.NewSource(seed))
	)
	t.Log("seed", seed)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)

	// server
	go func() {
		tcp, err := AcceptNoFinTCP(context.Background(), itun.WrapRawConn(s, 1536), time.Second)
		require.NoError(t, err)

		io.Copy(tcp, tcp)
	}()

	// client
	tcp, err := ConnectNoFinTCP(context.Background(), itun.WrapRawConn(c, 1536), time.Second)
	require.NoError(t, err)

	for i := 0; i < 64; i++ {
		var msg = make([]byte, r.Int31()%1024+1)
		r.Read(msg)

		_, err = tcp.Write(msg)
		require.NoError(t, err)

		var b = make([]byte, len(msg))
		_, err = io.ReadFull(tcp, b)
		require.NoError(t, err)

		require.Equal(t, string(msg), string(b), i)
	}
}

func Test_Ustack_TCP_Close(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP, test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP, test.RandPort())

		seed int64 = time.Now().UnixNano()
		r          = rand.New(rand.NewSource(seed))
	)
	t.Log("seed", seed)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)

	initNum, runNum := runtime.NumGoroutine(), 0

	// server
	serverCloseCh := make(chan struct{})
	go func() {
		defer func() { close(serverCloseCh) }()

		tcp, err := AcceptNoFinTCP(context.Background(), itun.WrapRawConn(s, 1536), time.Second)
		require.NoError(t, err)
		defer tcp.Close()

		_, err = io.Copy(tcp, tcp)
		require.NoError(t, err)
	}()

	{ // client
		tcp, err := ConnectNoFinTCP(context.Background(), itun.WrapRawConn(c, 1536), time.Second)
		require.NoError(t, err)

		runNum = runtime.NumGoroutine()

		{
			var msg = make([]byte, r.Int31()%1024+1)
			r.Read(msg)

			_, err = tcp.Write(msg)
			require.NoError(t, err)

			var b = make([]byte, len(msg))
			_, err = io.ReadFull(tcp, b)
			require.NoError(t, err)

			require.Equal(t, string(msg), string(b))
		}

		err = tcp.Close()
		require.NoError(t, err)
	}

	<-serverCloseCh
	time.Sleep(time.Second * 2)

	closedNum := runtime.NumGoroutine()

	msg := fmt.Sprintf("%d->%d->%d", initNum, runNum, closedNum)
	require.Equal(t, initNum, closedNum, msg)
}
