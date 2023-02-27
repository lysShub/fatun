package main

import (
	"fmt"
	"itun/divert"
	"net/netip"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
)

func main() {
	return

	h, err := divert.Open("outbound and !loopback and tcp", divert.LAYER_FLOW, 11, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		panic(err)
	}

	// 基于IP代理
	var b []byte
	var addr divert.Address
	for {
		if _, addr, err = h.Recv(b); err != nil {
			panic(err)
		} else {
			if addr.Header.Event != divert.EVENT_FLOW_ESTABLISHED {
				continue
			}

			f := addr.Flow()

			addr.Network()

			/*
				8532  curl.exe 127.0.0.1:56581      127.0.0.1:56581
				8532  curl.exe 192.168.43.162:56582 110.242.68.66:56582
				11776 TIM.exe
				7920  curl.exe 127.0.0.1:56587      127.0.0.1:56587
			*/

			name := getProcessName(int(f.ProcessId))

			fmt.Println(
				f.ProcessId,
				name,
				f.LocalAddr(),
				f.RemoteAddr(),
			)

			go func(laddr, raddr netip.AddrPort, pid int, pname string) {
				return
				if netBlock(laddr, raddr, pname) {
					fmt.Println(
						"block",
						pid,
						pname,
						laddr,
						raddr,
					)
				} else {
					fmt.Println(
						"work",
						pid,
						pname,
						laddr,
						raddr,
					)
				}
			}(f.LocalAddr(), f.RemoteAddr(), int(f.ProcessId), name)
		}
	}
}

func getProcessName(pid int) string {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		panic(err)
	}

	n, err := p.Name()
	if err != nil {
		panic(err)
	}
	return n
}

func netBlock(laddr, raddr netip.AddrPort, name string) (block bool) {
	var f = fmt.Sprintf("tcp and inbound and !loopback and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d", laddr.Addr(), laddr.Port(), raddr.Addr(), raddr.Port())

	fmt.Println(f)

	h, err := divert.Open(f, divert.LAYER_NETWORK, 9, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		panic(err)
	}
	// defer h.Close()

	block = true
	go func() {
		var b = make([]byte, 1536)
		var n int
		for {
			if n, _, err = h.Recv(b); err != nil {
				if !strings.Contains(err.Error(), "close") {
					panic(err)
				}
			} else if n > 0 {
				fmt.Println(name, "接受到", n)
				block = false
				return
			}
		}
	}()

	time.Sleep(time.Millisecond * 500)
	if block {
		if err := h.Shutdown(divert.WINDIVERT_SHUTDOWN_BOTH); err != nil {
			panic(err)
		}
	}

	return block
}
