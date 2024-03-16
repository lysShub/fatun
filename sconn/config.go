package sconn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lysShub/itun/crypto"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets PrevPackets //todo: support mutiple data set

	// swap secret key
	SwapKey crypto.SecretKey

	MTU int

	// todo: timeout
}

func (c *Config) init() error {
	if c == nil {
		return errors.New("xx")
	}

	return nil
}

type PrevPackets []header.TCP

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func (pps PrevPackets) Client(ctx context.Context, conn net.Conn) (err error) {
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
