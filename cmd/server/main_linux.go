//go:build linux
// +build linux

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/lysShub/fatcp"
	sconn "github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/crypto"
	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/server"
)

// go run -tags "-race debug"  .
// nohup go run -tags "-race debug" . &
func main() {

	var (
		proxy = ":443"
		// proxy = "172.24.131.26:19986"

		// tood: from config file
		cfg = &fatun.Config{
			Config: &fatcp.Config{
				Handshake: &fatun.Sign{
					PSS: func() fatun.PrevSegmets {
						var pss fatun.PrevSegmets
						if err := pss.Unmarshal("a.pss"); err != nil {
							panic(err)
						}
						return pss
					}(),

					Sign: &sconn.Sign{
						Sign: []byte("0123456789abcdef"),
						Parser: func(ctx context.Context, sign []byte) (crypto.Key, error) {
							return crypto.Key{9: 1}, nil
						},
					},
				},
			},

			Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
			// Logger: slog.New(slog.NewJSONHandler(func() *os.File {
			// 	fh, err := os.Create("server.log")
			// 	if err != nil {
			// 		panic(err)
			// 	}
			// 	return fh
			// }(), nil)),
		}
	)

	fmt.Println("启动")
	err := server.ListenAndServe(context.Background(), proxy, cfg)
	if err != nil {
		panic(err)
	}
}
