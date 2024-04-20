//go:build linux
// +build linux

package app_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/app/server"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/sockit/conn/tcp"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	// monkey.Patch(debug.Debug, func() bool { return false })

	cfg := &app.Config{
		Config: sconn.Config{
			PrevPackets:  pps,
			SwapKey:      sign,
			HandshakeMTU: 1460,
		},
		MTU:    1536,
		Logger: slog.NewJSONHandler(os.Stdout, nil),
	}

	raw, err := tcp.Listen(saddr)
	require.NoError(t, err)
	defer raw.Close()
	l, err := sconn.NewListener(raw, &cfg.Config)
	require.NoError(t, err)

	server.ListenAndServe(context.Background(), l, cfg)
}
