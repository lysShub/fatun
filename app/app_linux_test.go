package app_test

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/lysShub/itun/app/server"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/relraw/tcp/bpf"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	cfg := &server.Config{
		Config: config.Config{
			PrevPackets:      pps,
			HandShakeTimeout: ht,
			SwapKey:          &crypto.TokenServer{Valider: &tkServer{}},
		},

		MTU: 1536,

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
