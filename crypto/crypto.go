package crypto

import "github.com/lysShub/sockit/packet"

type Key = [Bytes]byte

type Crypto interface {
	Overhead() int

	Decrypt(pkt *packet.Packet) error
	DecryptRaw(ip *packet.Packet) error
	Encrypt(pkt *packet.Packet)
	EncryptRaw(ip *packet.Packet)
}
