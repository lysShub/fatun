package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

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
	if g.nonceLen > 12 {
		// checksum flag behind tcp[:12]
		return nil, fmt.Errorf("not support nonce length greater than header.TCPMinimumSize")
	}

	return g, nil
}

func (g *TCPCrypt) Encrypt(tcp header.TCP) header.TCP {
	n := len(tcp)
	for n+Bytes > cap(tcp) {
		tcp = append(tcp, 0)
	}
	tcp = tcp[:n]

	// todo: not strict nonce
	// additionalData can't contain tcp checksum flag
	i := tcp.DataOffset()
	g.c.Seal(tcp[i:i], tcp[:g.nonceLen], tcp[i:], tcp[:header.TCPChecksumOffset])

	tcp = tcp[:len(tcp)+Bytes]
	return tcp
}

// DecryptChecksum encrypt tcp packet and update checksum, pseudoSum1 indicate pseudo checksum
// with totalLen=0.
func (g *TCPCrypt) EncryptChecksum(tcp header.TCP, pseudoSum1 uint16) header.TCP {
	n := len(tcp)
	for n+Bytes > cap(tcp) {
		tcp = append(tcp, 0)
	}
	tcp = tcp[:n]

	i := tcp.DataOffset()
	g.c.Seal(tcp[i:i], tcp[:g.nonceLen], tcp[i:], tcp[:header.TCPChecksumOffset])

	tcp = tcp[:len(tcp)+Bytes]
	tcp.SetChecksum(0)
	ps := checksum.Combine(pseudoSum1, uint16(len(tcp)))
	tcp.SetChecksum(^checksum.Checksum(tcp, ps))
	return tcp
}

func (g *TCPCrypt) Decrypt(tcp header.TCP) (header.TCP, error) {
	i := tcp.DataOffset()

	_, err := g.c.Open(tcp[i:i], tcp[:g.nonceLen], tcp[i:], tcp[:header.TCPChecksumOffset])
	if err != nil {
		return nil, err
	}

	return tcp[:len(tcp)-Bytes], nil
}

// DecryptChecksum decrypt tcp packet and update checksum, pseudoSum1 indicate pseudo checksum
// with totalLen=0.
func (g *TCPCrypt) DecryptChecksum(tcp header.TCP, pseudoSum1 uint16) (header.TCP, error) {
	i := tcp.DataOffset()

	_, err := g.c.Open(tcp[i:i], tcp[:g.nonceLen], tcp[i:], tcp[:header.TCPChecksumOffset])
	if err != nil {
		return nil, err
	}

	tcp = tcp[:len(tcp)-Bytes]
	tcp.SetChecksum(0)
	tcp.SetChecksum(^checksum.Checksum(tcp, checksum.Combine(pseudoSum1, uint16(len(tcp)))))
	return tcp, nil
}
