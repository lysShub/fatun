//go:build linux
// +build linux

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
)

// go run -tags "-race debug"  .
// nohup go run -tags "-race debug" . &
func main() {

	var (
		proxy = ":443"
		// proxy = "172.24.131.26:19986"

		// tood: from config file
		cfg = &fatun.Config{
			Config: &sconn.Config{
				Key: &sconn.Sign{
					Sign: []byte("0123456789abcdef"),
					Parser: func(ctx context.Context, sign []byte) (crypto.Key, error) {
						return crypto.Key{9: 1}, nil
					},
				},
				PSS: func() sconn.PrevSegmets {
					var pss sconn.PrevSegmets
					if err := pss.Unmarshal("a.pss"); err != nil {
						panic(err)
					}
					return pss
				}(),
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
