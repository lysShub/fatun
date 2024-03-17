//go:build linux
// +build linux

package app_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/relraw/tcp/bpf"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	cfg := &app.Config{
		Config: sconn.Config{
			PrevPackets: pps,
			SwapKey:     &crypto.TokenServer{Valider: &tkServer{}},
			MTU:         1536,
		},
		Logger: slog.NewJSONHandler(os.Stdout, nil),
	}

	raw, err := bpf.Listen(saddr)
	require.NoError(t, err)
	defer raw.Close()
	l, err := sconn.NewListener(raw, &cfg.Config)
	require.NoError(t, err)

	server.ListenAndServe(context.Background(), l, cfg)
}
