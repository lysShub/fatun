package crypto

import "github.com/lysShub/netkit/packet"

type Key = [Bytes]byte

type Crypto interface {
	Overhead() int

	Decrypt(pkt *packet.Packet) error
	DecryptRaw(ip *packet.Packet) error
	Encrypt(pkt *packet.Packet)
	EncryptRaw(ip *packet.Packet)
}
