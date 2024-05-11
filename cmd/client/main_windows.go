//go:build windows
// +build windows

package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/lysShub/fatcp"
	sconn "github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/crypto"
	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/client"
)

// todo: support config

// go run -tags "-race debug"  .
func main() {
	/*
	    add launcher.exe
	   	add aces.exe
	*/

	fh, err := os.Create("client.log")
	if err != nil {
		panic(err)
	}
	defer fh.Close()

	var (
		// proxy = "172.24.131.26:443"
		proxy = "103.94.185.61:443"

		cfg = &fatun.Config{
			Config: &fatcp.Config{
				Handshake: &fatun.Sign{
					Sign: &sconn.Sign{
						Sign: []byte("0123456789abcdef"),
						Parser: func(ctx context.Context, sign []byte) (crypto.Key, error) {
							return crypto.Key{9: 1}, nil
						},
					},
					PSS: func() fatun.PrevSegmets {
						var pss fatun.PrevSegmets
						if err := pss.Unmarshal("a.pss"); err != nil {
							panic(err)
						}
						return pss
					}(),
				},

				MaxRecvBuffSize: 1536,
				MTU:             1500,
			},

			Logger: slog.New(slog.NewJSONHandler(fh, nil)),
		}
	)

	c, err := client.Proxy(context.Background(), proxy, cfg)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	var r = bufio.NewReader(os.Stdin)
	for {
		fmt.Print("->")

		str, err := r.ReadString('\n')
		if err != nil {
			panic(err)
		}
		str = strings.TrimSpace(str)

		ss := slices.Compact(strings.Split(str, " "))
		switch ctr := ss[0]; ctr {
		case "add", "del":
			if ctr == "add" {
				if err := c.Add(ss[1]); err != nil {
					fmt.Println("Error:\n", err.Error())
					return
				}
			} else {
				if err := c.Del(ss[1]); err != nil {
					fmt.Println("Error:\n", err.Error())
					return
				}
			}
		case "show":
			fmt.Println("filters: ", strings.Join(c.Filters(), ", "))
		case "pcap":
			if ss[1] == "del" {
				if err := c.DelPcap(); err != nil {
					fmt.Println("Error: ", err.Error())
				}
			} else {
				if err := c.AddPcap(ss[1]); err != nil {
					fmt.Println("Error:", err.Error())
				}
			}
		default:
			fmt.Println("无效参数", ctr)
		}
		fmt.Println()
	}

}
