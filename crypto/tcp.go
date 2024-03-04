package crypto

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type TCP struct {
	c cipher.AEAD

	enOffsetDelta uint32
	deOffsetDelta uint32
	pseudoSum1    uint16
}

const Bytes = 16
const nonces = 12

// NewTCP a tcp crypter, use AES-GCM
func NewTCP(key [Bytes]byte, pseudoSum1 uint16) (*TCP, error) {
	var g = &TCP{pseudoSum1: pseudoSum1}

	if block, err := aes.NewCipher(key[:]); err != nil {
		return nil, err
	} else {
		if g.c, err = cipher.NewGCM(block); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (g *TCP) Encrypt(tcp *relraw.Packet) {
	if debug.Debug() {
		test.ValidTCP(test.T(), tcp.Data(), g.pseudoSum1)
	}

	tcp.AllocTail(Bytes)
	tcphdr := header.TCP(tcp.Data())
	tcphdr.SetSequenceNumber(tcphdr.SequenceNumber() + g.enOffsetDelta)
	tcphdr.SetAckNumber(tcphdr.AckNumber() + g.enOffsetDelta)
	g.enOffsetDelta += Bytes

	i := tcphdr.DataOffset()
	g.c.Seal(tcphdr[i:i], tcphdr[:nonces], tcphdr[i:], tcphdr[:header.TCPChecksumOffset])

	tcphdr = tcphdr[:len(tcphdr)+Bytes]
	tcphdr.SetChecksum(0)
	psosum := checksum.Combine(g.pseudoSum1, uint16(len(tcphdr)))
	tcphdr.SetChecksum(^checksum.Checksum(tcphdr, psosum))

	tcp.SetLen(len(tcphdr))

	if debug.Debug() {
		test.ValidTCP(test.T(), tcp.Data(), g.pseudoSum1)
	}
}

func (g *TCP) EncryptRaw(ip *relraw.Packet) {
	if debug.Debug() {
		test.ValidIP(test.T(), ip.Data())
	}

	var hdrLen int
	var ver = header.IPVersion(ip.Data())
	switch ver {
	case 4:
		hdrLen = int(header.IPv4(ip.Data()).HeaderLength())
	case 6:
		hdrLen = header.IPv6MinimumSize
	default:
		panic("")
	}

	ip.SetHead(ip.Head() + hdrLen)
	g.Encrypt(ip)
	ip.SetHead(ip.Head() - hdrLen)

	if ver == 4 {
		iphdr := header.IPv4(ip.Data())
		iphdr.SetTotalLength(iphdr.TotalLength() + Bytes)
		sum := ^iphdr.Checksum()
		iphdr.SetChecksum(^checksum.Combine(sum, Bytes))
	} else {
		iphdr := header.IPv6(ip.Data())
		iphdr.SetPayloadLength(iphdr.PayloadLength() + Bytes)
	}

	if debug.Debug() {
		test.ValidIP(test.T(), ip.Data())
	}
}

func (g *TCP) Decrypt(tcp *relraw.Packet) error {
	if debug.Debug() {
		test.ValidTCP(test.T(), tcp.Data(), g.pseudoSum1)
	}

	tcphdr := header.TCP(tcp.Data())

	i := tcphdr.DataOffset()
	_, err := g.c.Open(tcphdr[i:i], tcphdr[:nonces], tcphdr[i:], tcphdr[:header.TCPChecksumOffset])
	if err != nil {
		return err
	}

	tcphdr = tcphdr[:len(tcphdr)-Bytes]
	tcphdr.SetSequenceNumber(tcphdr.SequenceNumber() - g.deOffsetDelta)
	tcphdr.SetAckNumber(tcphdr.AckNumber() - g.deOffsetDelta)
	g.deOffsetDelta += Bytes

	tcphdr.SetChecksum(0)
	psosum := checksum.Combine(g.pseudoSum1, uint16(len(tcphdr)))
	tcphdr.SetChecksum(^checksum.Checksum(tcphdr, psosum))

	tcp.SetLen(len(tcphdr))

	if debug.Debug() {
		test.ValidTCP(test.T(), tcp.Data(), g.pseudoSum1)
	}
	return nil
}

func (g *TCP) DecryptRaw(ip *relraw.Packet) error {
	if debug.Debug() {
		test.ValidIP(test.T(), ip.Data())
	}

	var hdrLen int
	var ver = header.IPVersion(ip.Data())
	switch ver {
	case 4:
		hdrLen = int(header.IPv4(ip.Data()).HeaderLength())
	case 6:
		hdrLen = header.IPv6MinimumSize
	default:
		panic("")
	}

	ip.SetHead(ip.Head() + hdrLen)
	if err := g.Decrypt(ip); err != nil {
		return err
	}
	ip.SetHead(ip.Head() - hdrLen)

	if ver == 4 {
		iphdr := header.IPv4(ip.Data())
		iphdr.SetTotalLength(iphdr.TotalLength() - Bytes)
		sum := ^iphdr.Checksum()
		iphdr.SetChecksum(^checksum.Combine(sum, ^uint16(Bytes)))
	} else {
		iphdr := header.IPv6(ip.Data())
		iphdr.SetPayloadLength(iphdr.PayloadLength() - Bytes)
	}

	if debug.Debug() {
		test.ValidIP(test.T(), ip.Data())
	}
	return nil
}
