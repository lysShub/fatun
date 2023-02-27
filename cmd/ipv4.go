package main

import (
	"encoding/binary"
	"net"

	"golang.org/x/net/ipv4"
)

// TODO: 部分IP附加一个Header后, 可能溢出, 导致传输失败; 暂时不管
func IP4OnIP4(ipPack []byte, newSrc, newDst net.IP) ([]byte, error) {
	var oh = &ipv4.Header{}
	if err := oh.Parse(ipPack); err != nil {
		return nil, err
	}

	oh.Len = 5
	oh.TotalLen += 20
	oh.Checksum = 0
	oh.Src = newSrc
	oh.Dst = newDst

	if b, err := oh.Marshal(); err != nil {
		return nil, err
	} else {
		oh.Checksum = int(checkSum(b))
		b, _ = oh.Marshal()
		return append(b, ipPack...), nil
	}
}

func tcpSyn(lport, rport uint16) []byte {

	var b = make([]byte, 0, 20)

	binary.LittleEndian.AppendUint16(b, lport)
	binary.LittleEndian.AppendUint16(b, rport)

	return nil
}

func checkSum(d []byte) uint16 {
	var S uint32
	l := len(d)
	if l&0b1 == 1 {
		for i := 0; i < l-1; {
			S = S + uint32(d[i])<<8 + uint32(d[i+1])
			if S>>16 > 0 {
				S = S&0xffff + 1
			}
			i = i + 2
		}
		S = S + uint32(d[l-1])<<8
	} else {
		for i := 0; i < l; {
			S = S + uint32(d[i])<<8 + uint32(d[i+1])
			if S>>16 > 0 {
				S = S&0xffff + 1
			}
			i = i + 2
		}
	}

	return uint16(65535) - uint16(S)
}
