package app

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/lysShub/itun/app/server"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/relraw/tcp/bpf"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	cfg := &server.Config{
		Sconn: sconn.Config{
			PrevPackets:      pps,
			HandShakeTimeout: ht,
			SwapKey:          &sconn.TokenServer{Valider: &tkServer{}},
		},
		MTU:                 1536,
		TCPHandshakeTimeout: ht,
		InitCfgTimeout:      ht,
		ProxyerIdeleTimeout: ht,
	}

	l, err := bpf.Listen(saddr)
	require.NoError(t, err)
	defer l.Close()

	server.ListenAndServe(context.Background(), l, cfg)
}

func TestTCP(t *testing.T) {
	addr := &net.TCPAddr{IP: saddr.Addr().AsSlice(), Port: int(saddr.Port())}
	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)

	conn, err := l.AcceptTCP()
	require.NoError(t, err)
	defer conn.Close()

	io.Copy(conn, conn)
}
