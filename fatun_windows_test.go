//go:build windows
// +build windows

package fatun_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "net/http/pprof"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/client"
	"github.com/stretchr/testify/require"
)

func TestXxxxx(t *testing.T) {

	var (
		cfg = &fatun.Config{
			Config: cfg,

			Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		}
	)

	c, err := client.Proxy(context.Background(), "172.24.131.26:443", cfg)
	require.NoError(t, err)
	defer c.Close()

	err = c.Add("chrome.exe")
	// err = c.Add("curl.exe")
	// err = c.Add(filter.DefaultFilter)
	require.NoError(t, err)

	time.Sleep(time.Hour)
}
