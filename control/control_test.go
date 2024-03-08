package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"runtime"
	"testing"
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
)

func Test_Control(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
	)
	parentCtx := cctx.WithContext(context.Background())
	fmt.Println(caddr, saddr)

	// server
	go func() {
		// ctx := cctx.WithContext(parentCtx)

		// var tcp net.Conn

		// Serve(ctx, tcp, &mockServer{})
	}()

	// client
	{
		ctx := cctx.WithContext(parentCtx)

		var tcp net.Conn

		client := NewClient(tcp)
		require.NoError(t, ctx.Err())
		defer client.Close()

		ipv6, err := client.IPv6(ctx)
		require.NoError(t, err)
		require.True(t, ipv6)
	}
}

func Test_Control_Client_Close(t *testing.T) {

	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
	)
	parentCtx := cctx.WithContext(context.Background())
	var rets []int

	fmt.Println(caddr, saddr)

	initNum, runNum := runtime.NumGoroutine(), 0

	// server
	go func() {
		ctx := cctx.WithContext(parentCtx)

		// var tcp net.Conn

		// Serve(ctx, tcp, &mockServer{})

		<-ctx.Done()
		require.True(t, errors.Is(ctx.Err(), io.EOF))
		rets = append(rets, 3)
	}()

	// client
	go func() {
		ctx := cctx.WithContext(parentCtx)

		var tcp net.Conn

		runNum = runtime.NumGoroutine()

		client := NewClient(tcp)
		require.NoError(t, ctx.Err())
		defer client.Close()

		ipv6, err := client.IPv6(ctx)
		require.NoError(t, err)
		require.True(t, ipv6)

		err = client.Close()
		require.NoError(t, err)

		<-ctx.Done()
		require.True(t, errors.Is(ctx.Err(), context.Canceled))
		rets = append(rets, 6)
	}()

	time.Sleep(time.Second * 5)
	endNum := runtime.NumGoroutine()

	require.Equal(t, 6, len(rets), rets)
	require.Equal(t, initNum, endNum, fmt.Sprintf("%d->%d->%d", initNum, runNum, endNum))
}

func Test_Control_Server_Close(t *testing.T) {
	t.Skip("todo")
}

type mockServer struct {
	InitedCfg bool
}

var _ Handler = (*mockServer)(nil)

func (h *mockServer) IPv6() bool {
	return true
}
func (h *mockServer) EndConfig() {
	h.InitedCfg = true
}
func (h *mockServer) AddSession(s session.Session) (session.ID, error) {
	return 1, nil
}
func (h *mockServer) DelSession(id session.ID) error {
	return nil
}
func (h *mockServer) PackLoss() float32 { return 0.01 }
func (h *mockServer) Ping()             {}
