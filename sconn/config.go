package sconn

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"

	"github.com/lysShub/itun/crypto"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets PrevPackets //todo: support mutiple data set

	// swap secret key
	SwapKey SwapKey

	HandshakeMTU int
}

func (c *Config) init() error {
	if c == nil {
		return errors.New("xx")
	}

	if c.HandshakeMTU <= 0 {
		return errors.New("invalid mtu")
	}

	return nil
}

type PrevPackets []header.TCP

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func (pps PrevPackets) Client(ctx context.Context, conn net.Conn) (err error) {
	var ret = make(chan struct{})
	var canceled bool
	defer func() {
		if canceled {
			err = ctx.Err()
		}
		close(ret)
	}()
	go func() {
		select {
		case <-ctx.Done():
			canceled = true
			conn.SetDeadline(time.Now())
		case <-ret:
		}
	}()

	for i := 0; i < len(pps); i++ {
		if i%2 == 0 {
			_, err := conn.Write(pps[i])
			if err != nil {
				return err
			}
		} else {
			var b = make([]byte, len(pps[i]))

			if _, err := io.ReadFull(conn, b); err != nil {
				return err
			}
			if !bytes.Equal(b, pps[i]) {
				return ErrPrevPacketInvalid(i)
			}
		}
	}
	return nil
}

func (pps PrevPackets) Server(ctx context.Context, conn net.Conn) (err error) {
	var retCh = make(chan struct{})
	var canceled bool
	defer func() {
		if canceled {
			err = ctx.Err()
		}
		close(retCh)
	}()
	go func() {
		select {
		case <-ctx.Done():
			canceled = true
			conn.SetDeadline(time.Now())
		case <-retCh:
		}
	}()

	for i := 0; i < len(pps); i++ {
		if i%2 == 0 {
			var b = make([]byte, len(pps[i]))

			if _, err := io.ReadFull(conn, b); err != nil {
				return err
			}
			if !bytes.Equal(b, pps[i]) {
				return ErrPrevPacketInvalid(i)
			}
		} else {
			_, err := conn.Write(pps[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type SwapKey interface {
	Client(ctx context.Context, conn net.Conn) (crypto.Key, error)
	Server(ctx context.Context, conn net.Conn) (crypto.Key, error)
}

// Sign sign can't guarantee transport security
type Sign struct {
	Sign   []byte
	Parser func(sign []byte) (crypto.Key, error)
}

func (t *Sign) Client(ctx context.Context, conn net.Conn) (crypto.Key, error) {
	key, err := t.Parser(t.Sign)
	if err != nil {
		return crypto.Key{}, err
	}

	var ret = make(chan struct{})
	defer close(ret)
	go func() {
		select {
		case <-ctx.Done():
			conn.SetDeadline(time.Now())
		case <-ret:
			return
		}
	}()

	err = gob.NewEncoder(conn).Encode(t.Sign)
	if err != nil {
		return crypto.Key{}, errors.WithStack(err)
	}

	return key, nil
}

func (t Sign) Server(ctx context.Context, conn net.Conn) (crypto.Key, error) {
	var ret = make(chan struct{})
	defer close(ret)
	go func() {
		select {
		case <-ctx.Done():
			conn.SetDeadline(time.Now())
		case <-ret:
			return
		}
	}()

	var sign []byte
	err := gob.NewDecoder(conn).Decode(&sign)
	if err != nil {
		return crypto.Key{}, errors.WithStack(err)
	}

	return t.Parser(sign)
}
