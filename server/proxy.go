package server

import (
	"itun/pack"
	"net"
)

type proxy struct {
	clientConn net.Conn
	clientIP   [4]byte
	portMgr    *PortMgr

	ipHdr *pack.IPHdr

	dstIPs map[[4]byte]struct{}
}

func (p *proxy) proxy() {

	var b = make([]byte, 1532)
	var n int
	var err error
	var dstIP [4]byte
	for {
		if n, err = p.clientConn.Read(b[20:]); err != nil {
			panic(err)
		} else {
			dstIP = pack.Parse(b[:n])

			p.ipHdr.SetTotalLen(uint16(n - pack.W + 20))
			p.ipHdr.SetID(0)
			p.ipHdr.SetSrcIP(p.clientIP)
			p.ipHdr.SetChecksum(0)
			p.ipHdr.SetChecksum(pack.Checksum(*p.ipHdr))

			copy(b[0:], (*p.ipHdr)[:])

		}
	}
}

func (p *proxy) inject(dstIP [4]byte, sPort, cPort uint16) {

}
