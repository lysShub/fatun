package crypto

import (
	"context"
	"encoding/gob"
	"io"
	"net"

	"github.com/pkg/errors"
)

type SecretKey interface {
	// SecretKey  get crypto secret key, return Key{} mean not crypto
	SecretKey(ctx context.Context, conn net.Conn) (Key, error)
}

type Key = [Bytes]byte

type NotCryptoClient struct{}
type NotCryptoServer struct{}

var _ SecretKey = (*NotCryptoClient)(nil)

func (c *NotCryptoClient) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	var key = Key{}

	n, err := conn.Write(key[:])
	if err != nil {
		return key, err
	} else if n != Bytes {
		return key, errors.Errorf("SecretKey write interrupt")
	}

	n, err = io.ReadFull(conn, key[:])
	if err != nil {
		return key, err
	} else if n != Bytes {
		return key, errors.Errorf("SecretKey read interrupt")
	}

	if key != (Key{}) {
		return key, errors.Errorf("SecretKey NotCrypto faild")
	}

	return key, nil
}

func (c *NotCryptoServer) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	var key = (Key{})

	n, err := io.ReadFull(conn, key[:])
	if err != nil {
		return key, err
	} else if n != Bytes {
		return key, errors.Errorf("SecretKey read interrupt")
	}

	if key != (Key{}) {
		return key, errors.Errorf("SecretKey NotCrypto faild")
	}

	n, err = conn.Write(key[:])
	if err != nil {
		return key, err
	} else if n != Bytes {
		return key, errors.Errorf("SecretKey write interrupt")
	}

	return key, nil
}

type TokenClient struct {
	Tokener interface {
		Token() (tk []byte, key Key, err error)
	}
}

type TokenServer struct {
	Valider interface {
		Valid(tk []byte) (key Key, err error)
	}
}

func (c *TokenClient) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	tk, key, err := c.Tokener.Token()
	if err != nil {
		return Key{}, err
	}

	err = gob.NewEncoder(conn).Encode(tk)
	if err != nil {
		return Key{}, err
	}

	var resp string
	err = gob.NewDecoder(conn).Decode(&resp)
	if err != nil {
		return Key{}, err
	}

	if resp != "" {
		return Key{}, errors.Errorf("SecretKey Token faild, %s", resp)
	}
	return key, nil
}

func (c *TokenServer) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	var req []byte
	err := gob.NewDecoder(conn).Decode(&req)
	if err != nil {
		return Key{}, err
	}

	var resp string
	key, err := c.Valider.Valid(req)
	if err != nil {
		resp = err.Error()
	}

	err = gob.NewEncoder(conn).Encode(resp)
	if err != nil {
		return Key{}, err
	}

	return key, nil
}
