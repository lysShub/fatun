package proxy

import (
	"fmt"
	"itun/pack"
	"net"

	"github.com/lysShub/go-divert"
)

type state uint8

const (
	idle state = iota
	once
	work
	clog // proxy ing
)

type captur struct {
	localPort uint16
	dstIP     [4]byte

	handle    divert.Handle
	proxyConn net.Conn
}

func (p *captur) proxy(secSyn []byte) {
	_, err := p.proxyConn.Write(pack.Packe(secSyn, p.dstIP))
	if err != nil {
		return
	}

	var f = fmt.Sprintf("ip and !loopback and outbound and localPort=%d", p.localPort)
	p.handle, err = divert.Open(f, divert.LAYER_NETWORK, capturPri, 0)
	if err != nil {
		return
	}

	var b = make([]byte, 1536)
	var n int
	for {
		if n, _, err = p.handle.Recv(b); err != nil {
			return
		} else {
			ipHdrLen := int(b[0]>>4) * 4

			if _, err = p.proxyConn.Write(pack.Packe(b[ipHdrLen:n], p.dstIP)); err != nil {
				return
			}
		}
	}
}
