//go:build windows
// +build windows

package fatun_test

import (
	"context"
	"testing"
	"time"

	_ "net/http/pprof"

	"github.com/lysShub/fatun/config"
	"github.com/lysShub/fatun/fatun/client"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
	"github.com/stretchr/testify/require"
)

func TestXxxxx(t *testing.T) {
	var cfg = &config.Config{
		MTU: 1536,
		PSS: `./a.pss`,
		Key: &sconn.Sign{
			Sign: []byte("0123456789abcdef"),
			Parser: func(sign []byte) (crypto.Key, error) {
				return crypto.Key{9: 1}, nil
			},
		},
		// Log: "stdout",
	}

	acfg, err := cfg.Config()
	require.NoError(t, err)

	c, err := client.Proxy(context.Background(), "172.24.131.26:443", acfg)
	require.NoError(t, err)
	defer c.Close()

	err = c.Add("chrome.exe")
	// err = c.Add("curl.exe")
	// err = c.Add(filter.DefaultFilter)
	require.NoError(t, err)

	time.Sleep(time.Hour)
}
