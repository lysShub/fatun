package main

import (
	"fmt"
	"net"

	"github.com/lysShub/go-divert"

	"golang.org/x/net/dns/dnsmessage"
)

var dnsServer = net.IPv4(8, 8, 8, 8)

func Dns() {
	f := "udp and outbound and !loopback and remotePort==53"
	h, err := divert.Open(f, divert.LAYER_NETWORK, 21, divert.FLAG_READ_ONLY)
	if err != nil {
		panic(err)
	}

	var da = make([]byte, 1500)
	var n int
	for {

		if n, _, err = h.Recv(da); err != nil {
			panic(err)
		} else if n > 0 {

		}
	}

}

func pacpDNS() {
	f := "udp and outbound and !loopback and remotePort==53"
	h, err := divert.Open(f, divert.LAYER_NETWORK, 21, divert.FLAG_READ_ONLY)
	if err != nil {
		panic(err)
	}

	var da = make([]byte, 1500)
	var n int
	// var addr divert.Address
	for {

		if n, _, err = h.Recv(da); err != nil {
			panic(err)
		} else if n > 0 {
			da = da[28:]

			var p dnsmessage.Parser
			if _, err := p.Start(da); err != nil {
				panic(err)
			}
			for {
				q, err := p.Question()
				if err == dnsmessage.ErrSectionDone {
					break
				}
				if err != nil {
					panic(err)
				}

				fmt.Println(q.Name.String())
			}

		}

	}
}
