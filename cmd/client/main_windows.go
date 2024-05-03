//go:build windows
// +build windows

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/lysShub/fatun/config"
	"github.com/lysShub/fatun/fatun/client"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/sconn/crypto"
)

// todo: support config

// go run -tags  debug  .
func main() {
	/*
	    add launcher.exe
	   	add aces.exe
	*/

	cfg := &config.Config{
		// Server: "172.24.131.26:443",

		MTU: 1536,
		Key: &sconn.Sign{
			Sign: []byte("0123456789abcdef"),
			Parser: func(sign []byte) (crypto.Key, error) {
				return crypto.Key{9: 1}, nil
			},
		},
		PSS: "a.pss",
		Log: "client.log",
	}

	acfg, err := cfg.Config()
	if err != nil {
		panic(err)
	}

	c, err := client.Proxy(context.Background(), cfg.Server, acfg)
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
		default:
			fmt.Println("无效参数", ctr)
		}
		fmt.Println()
	}

}
