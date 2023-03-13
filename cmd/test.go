package main

import (
	"fmt"

	"github.com/lysShub/go-divert"
)

// 分叉DNS数据包到  8.8.8.8
//
//	TODO: 后续做
func redirect() {

	var f = "udp and !loopback and (remotePort==53 or localPort==53)"

	h, err := divert.Open(f, divert.LAYER_NETWORK, 31, divert.FLAG_READ_ONLY|divert.FLAG_WRITE_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		panic(err.Error())
	}

	var b = make([]byte, 512)
	var n int
	var addr divert.Address
	for {

		if n, addr, err = h.Recv(b); err != nil {
			panic(err.Error())
		} else {

			if addr.Header.Outbound() {

			} else {

			}

			fmt.Println(n)
		}
	}

}
