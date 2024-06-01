package conn

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type key = [16]byte

const bytes = 16

// 加密不包括peer头
// 加密不包括builtin包（其实可以包括）
type crypto struct {
	c          cipher.AEAD
	headerSize int
}

func newCrypto(key key, headerSize int) (*crypto, error) {
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

func (c *crypto) encrypt(seg *packet.Packet) {
	b := seg.AppendN(bytes).ReduceN(bytes).Bytes()

	i := c.headerSize
	c.c.Seal(b[i:i], b[:i], b[i:], nil)
	seg.SetData(seg.Data() + bytes)
}

func (c *crypto) decrypt(seg *packet.Packet) error {
	b := seg.Bytes()
	if len(b) < c.headerSize+bytes {
		return errors.New("decrypt invalid packet")
	}

	i := c.headerSize
	_, err := c.c.Open(b[i:i], b[:i], b[i:], nil)
	if err != nil {
		return errors.WithStack(err)
	}
	seg.SetData(seg.Data() - bytes)

	return nil
}
