//go:build linux
// +build linux

package fatun_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/server"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	// monkey.Patch(debug.Debug, func() bool { return false })

	cfg := &fatun.Config{
		Config: cfg,
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	err := server.ListenAndServe(context.Background(), ":443", cfg)
	require.NoError(t, err)
}
