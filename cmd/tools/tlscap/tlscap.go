//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	nurl "net/url"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/lysShub/fatun/app/client/capture"
	"github.com/lysShub/fatun/config"
)

func main() {
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	} else {
		args = []string{"-h"}
	}
	if args[0] == "-h" || len(args) != 3 {
		fmt.Println("tlscap config-file-path url bytes-size")
		fmt.Println("example:")
		fmt.Println("tlscap ./config.go https://dlcdn.apache.org/maven/maven-3/3.8.5/binaries/apache-maven-3.8.5-bin.zip 32kB")
		return
	}

	var (
		cfg  = &config.Config{}
		url  string
		size int
	)

	if path, err := filepath.Abs(args[0]); err != nil {
		fmt.Println("config-file-path:", err.Error())
		return
	} else {
		if err := cfg.Load(path); err != nil {
			fmt.Println("config-file-path:", err.Error())
			return
		}
		defer cfg.Flush(path)
	}

	if _, err := nurl.Parse(args[1]); err != nil {
		fmt.Println("url:", err.Error())
		return
	} else {
		url = args[1]
	}

	if v, err := units.RAMInBytes(args[2]); err != nil {
		fmt.Println("bytes-size:", err.Error())
		return
	} else {
		size = int(v)
	}

	pps, err := capture.CaptureTLSWithGolang(context.Background(), url, size)
	if err != nil {
		fmt.Println("capture:", err.Error())
		return
	} else {
		cfg.PrevPackets = pps
	}
}
