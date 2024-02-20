package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type TCPCrypt struct {
	c        cipher.AEAD
	nonceLen int
}

const Bytes = 16

// NewTCPCrypt a tcp crypter, use AES-GCM
func NewTCPCrypt(key [Bytes]byte) (*TCPCrypt, error) {
	var g = &TCPCrypt{}

	if block, err := aes.NewCipher(key[:]); err != nil {
		return nil, err
	} else {
		if g.c, err = cipher.NewGCM(block); err != nil {
			return nil, err
		}
	}
	g.nonceLen = g.c.NonceSize()
	if g.nonceLen > header.TCPChecksumOffset {
		return nil, fmt.Errorf("not support nonce length greater than header.TCPChecksumOffset")
	}

	return g, nil
}

func (g *TCPCrypt) Encrypt(tcp *relraw.Packet) {
	tcp.AllocTail(Bytes)

	// todo: not strict nonce

	// additionalData can't contain tcp checksum flag
	tcphdr := header.TCP(tcp.Data())
	i := tcphdr.DataOffset()
	g.c.Seal(tcphdr[i:i], tcphdr[:g.nonceLen], tcphdr[i:], tcphdr[:header.TCPChecksumOffset])

	tcp.SetLen(len(tcphdr) + Bytes)
}

// DecryptChecksum encrypt tcp packet and update checksum, pseudoSum1 indicate pseudo checksum
// with totalLen=0.
func (g *TCPCrypt) EncryptChecksum(tcp *relraw.Packet, pseudoSum1 uint16) {
	tcp.AllocTail(Bytes)

	tcphdr := header.TCP(tcp.Data())

	i := tcphdr.DataOffset()
	g.c.Seal(tcphdr[i:i], tcphdr[:g.nonceLen], tcphdr[i:], tcphdr[:header.TCPChecksumOffset])

	tcphdr = tcphdr[:len(tcphdr)+Bytes]
	tcphdr.SetChecksum(0)
	ps := checksum.Combine(pseudoSum1, uint16(len(tcphdr)))
	tcphdr.SetChecksum(^checksum.Checksum(tcphdr, ps))

	tcp.SetLen(len(tcphdr))
}

func (g *TCPCrypt) Decrypt(tcp *relraw.Packet) error {
	tcphdr := header.TCP(tcp.Data())

	i := tcphdr.DataOffset()
	_, err := g.c.Open(tcphdr[i:i], tcphdr[:g.nonceLen], tcphdr[i:], tcphdr[:header.TCPChecksumOffset])
	if err != nil {
		return err
	}

	tcp.SetLen(len(tcphdr) - Bytes)
	return nil
}

// DecryptChecksum decrypt tcp packet and update checksum, pseudoSum1 indicate pseudo checksum
// with totalLen=0.
func (g *TCPCrypt) DecryptChecksum(tcp *relraw.Packet, pseudoSum1 uint16) error {
	tcphdr := header.TCP(tcp.Data())

	i := tcphdr.DataOffset()
	_, err := g.c.Open(tcphdr[i:i], tcphdr[:g.nonceLen], tcphdr[i:], tcphdr[:header.TCPChecksumOffset])
	if err != nil {
		return err
	}

	tcphdr = tcphdr[:len(tcphdr)-Bytes]
	tcphdr.SetChecksum(0)
	tcphdr.SetChecksum(^checksum.Checksum(tcphdr, checksum.Combine(pseudoSum1, uint16(len(tcphdr)))))

	tcp.SetLen(len(tcphdr))
	return nil
}
