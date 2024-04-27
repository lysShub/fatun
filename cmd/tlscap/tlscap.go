//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	nurl "net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/go-units"
	"github.com/lysShub/fatun/fatun/tools"
)

func main() {
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	} else {
		args = []string{"-h"}
	}
	if args[0] == "-h" || len(args) != 4 {
		fmt.Print(helper)
		return
	}

	var (
		url      string
		size     int
		mssDelta int // = -sconn.Overhead
		path     string
	)

	if _, err := nurl.Parse(args[0]); err != nil {
		fmt.Println("url:", err.Error())
		return
	} else {
		url = args[0]
	}

	if v, err := units.RAMInBytes(args[1]); err != nil {
		fmt.Println("bytes-size:", err.Error())
		return
	} else {
		size = int(v)
	}

	if v, err := strconv.Atoi(args[2]); err != nil {
		fmt.Println("mss-delta:", err.Error())
		return
	} else {
		mssDelta = v
	}

	if v, err := filepath.Abs(args[3]); err != nil {
		fmt.Println("pss-path:", err.Error())
		return
	} else {
		path = v
	}

	pss, err := tools.CaptureTLSWithGolang(context.Background(), url, size, mssDelta)
	if err != nil {
		fmt.Println("capture:", err.Error())
		return
	} else {
		err := pss.Marshal(path)
		if err != nil {
			fmt.Println("error:", err.Error())
		}
	}
	fmt.Println("success.")
	fmt.Println(path)
}

const (
	helper = `
tlscap url bytes-size mss-delta pss-path

capture the first bytes-size TCP segments for the URL http(s) GET request, store them in 
pss-path, where mss-delta can be used to modify the TCP MSS.

example:
tlscap https://example.com/xxx-1.2.3-bin.zip 32kB -16 ./apache.pss
`
)
