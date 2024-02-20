package sconn

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"net"

	"github.com/lysShub/itun/sconn/crypto"
)

type SecretKeyClient interface {
	SecretKey
	client()
}
type clientImpl struct{}

func (clientImpl) client() {}

type SecretKeyServer interface {
	SecretKey
	server()
}

type serverImpl struct{}

func (serverImpl) server() {}

type SecretKey interface {

	// return [crypto.Bytes]byte{} mean not crypto
	SecretKey(ctx context.Context, conn net.Conn) (Key, error)
}

type Key = [crypto.Bytes]byte

type NotCryptoClient struct{ clientImpl }
type NotCryptoServer struct{ serverImpl }

var _ SecretKey = (*NotCryptoClient)(nil)

func (c *NotCryptoClient) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	var key = Key{}

	n, err := conn.Write(key[:])
	if err != nil {
		return key, err
	} else if n != crypto.Bytes {
		return key, fmt.Errorf("SecretKey write interrupt")
	}

	n, err = io.ReadFull(conn, key[:])
	if err != nil {
		return key, err
	} else if n != crypto.Bytes {
		return key, fmt.Errorf("SecretKey read interrupt")
	}

	if key != (Key{}) {
		return key, fmt.Errorf("SecretKey NotCrypto faild")
	}

	return key, nil
}

func (c *NotCryptoServer) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	var key = (Key{})

	n, err := io.ReadFull(conn, key[:])
	if err != nil {
		return key, err
	} else if n != crypto.Bytes {
		return key, fmt.Errorf("SecretKey read interrupt")
	}

	if key != (Key{}) {
		return key, fmt.Errorf("SecretKey NotCrypto faild")
	}

	n, err = conn.Write(key[:])
	if err != nil {
		return key, err
	} else if n != crypto.Bytes {
		return key, fmt.Errorf("SecretKey write interrupt")
	}

	return key, nil
}

// jwt etc.
type TokenClient struct {
	clientImpl
	Tokener interface {
		Token() (tk []byte, key Key, err error)
	}
}

type TokenServer struct {
	serverImpl
	Valider interface {
		Valid(tk []byte) (key Key, err error)
	}
}

type TokenResp struct {
	OK  bool
	Err string
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

	var resp = &TokenResp{}
	err = gob.NewEncoder(conn).Encode(&resp)
	if err != nil {
		return Key{}, err
	}

	if !resp.OK {
		return Key{}, fmt.Errorf("SecretKey Token faild, %s", resp.Err)
	}
	return key, nil
}

func (c *TokenServer) SecretKey(ctx context.Context, conn net.Conn) (Key, error) {
	var req []byte
	err := gob.NewEncoder(conn).Encode(&req)
	if err != nil {
		return Key{}, err
	}

	var resp *TokenResp
	key, err := c.Valider.Valid(req)
	if err != nil {
		resp = &TokenResp{
			OK:  false,
			Err: err.Error(),
		}
	} else {
		resp = &TokenResp{OK: true}
	}

	err = gob.NewDecoder(conn).Decode(resp)
	if err != nil {
		return Key{}, err
	}

	return key, nil
}
