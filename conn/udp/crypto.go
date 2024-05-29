package udp

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

// 加密不包括peer头
// 加密不包括builtin包
// 加密在peer Encode 之后，解密在peer Decode 之后， peer部分数据作为nonce
type Key = [16]byte

const Bytes = 16

type crypto struct {
	c          cipher.AEAD
	headerSize int
}

func NewCrypto(key Key, headerSize int) (*crypto, error) {
	var c = &crypto{headerSize: headerSize}
	if block, err := aes.NewCipher(key[:]); err != nil {
		return nil, err
	} else {
		if c.c, err = cipher.NewGCMWithNonceSize(block, headerSize); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *crypto) Encrypt(seg *packet.Packet) {
	b := seg.AppendN(Bytes).ReduceN(Bytes).Bytes()

	i := c.headerSize
	c.c.Seal(b[i:i], b[:i], b[i:], nil)
	seg.SetData(seg.Data() + Bytes)
}

func (c *crypto) Decrypt(seg *packet.Packet) error {
	b := seg.Bytes()
	if len(b) < c.headerSize+Bytes {
		return errors.New("decrypt invalid packet")
	}

	i := c.headerSize
	_, err := c.c.Open(b[i:i], b[:i], b[i:], nil)
	if err != nil {
		return errors.WithStack(err)
	}
	seg.SetData(seg.Data() - Bytes)

	return nil
}
