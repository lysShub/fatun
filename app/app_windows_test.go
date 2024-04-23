//go:build windows
// +build windows

package app_test

import (
	"context"
	"testing"
	"time"

	_ "net/http/pprof"

	"github.com/lysShub/fatun/app/client"
	"github.com/lysShub/fatun/config"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
	"github.com/stretchr/testify/require"
)

func TestXxxxx(t *testing.T) {
	var cfg = &config.Config{
		MTU:         1536,
		PrevPackets: `./tools/a.pps`,
		SwapKey: &sconn.Sign{
			Sign: []byte("0123456789abcdef"),
			Parser: func(sign []byte) (crypto.Key, error) {
				return crypto.Key{9: 1}, nil
			},
		},
		Log: "stdout",
	}

	acfg, err := cfg.Config()
	require.NoError(t, err)

	c, err := client.Proxy(context.Background(), "172.24.131.26:8080", acfg)
	require.NoError(t, err)
	defer c.Close()

	err = c.AddProcess("chrome.exe")
	// err = c.EnableDefault()
	require.NoError(t, err)

	time.Sleep(time.Hour)
}
