//go:build windows
// +build windows

package main

import (
	"context"

	"github.com/lysShub/fatun/app/client"
	"github.com/lysShub/fatun/config"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
)

func main() {
	cfg := &config.Config{
		Server: "172.24.131.26:19986",
		MTU:    1536,
		SwapKey: &sconn.Sign{
			Sign: []byte("0123456789abcdef"),
			Parser: func(sign []byte) (crypto.Key, error) {
				return crypto.Key{9: 1}, nil
			},
		},
		PrevPackets: "../a.pps",
		Log:         "../client.log",
	}

	acfg, err := cfg.Config()
	if err != nil {
		panic(err)
	}

	cc, err := client.Proxy(context.Background(), cfg.Server, acfg)
	if err != nil {
		panic(err)
	}

	cc.DisableDefault()

}
