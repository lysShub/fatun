package server

import (
	"itun/pack"
	"net"
)

// 代表一个被代理的用户
type user struct {
	clientConn net.Conn
	clientIP   [4]byte
	portMgr    *PortMgr

	conns []*proxyConn
}

type proxyConn struct {
	dstIP             [4]byte
	dstPort           uint16
	pryPort           uint16
	outIpHdr, inIpHdr *pack.IPHdr

	inConn net.PacketConn
}

func (p *user) proxy() {

	var b = make([]byte, 1532)
	var n int
	var err error
	var dstIP [4]byte
	for {
		if n, err = p.clientConn.Read(b[20:]); err != nil {
			panic(err)
		} else if n >= 20 {
			dstIP = pack.Parse(b[:n])

			const reserved = 12
			if b[20+reserved]&0b111 == 0 {

				p.ipHdr.SetTotalLen(uint16(n - pack.W + 20))
				p.ipHdr.SetID(0)
				p.ipHdr.SetSrcIP(p.clientIP)
				p.ipHdr.SetChecksum(0)
				p.ipHdr.SetChecksum(pack.Checksum(*p.ipHdr))

				copy(b[0:], (*p.ipHdr)[:])

			} else {

			}
		}
	}
}

func (p *user) inject(dstIP [4]byte, sPort, cPort uint16) {

}
