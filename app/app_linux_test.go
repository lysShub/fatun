//go:build linux
// +build linux

package app_test

import (
	"context"
	"testing"

	"github.com/lysShub/fatun/app/server"
	"github.com/lysShub/fatun/config"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	// monkey.Patch(debug.Debug, func() bool { return false })

	cfg := &config.Config{
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

	c, err := cfg.Config()
	require.NoError(t, err)

	err = server.ListenAndServe(context.Background(), ":8080", c)
	require.NoError(t, err)
}
