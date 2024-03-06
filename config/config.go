package config

import (
	"bytes"
	"context"
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

	HandShakeTimeout time.Duration

	// swap secret key
	SwapKey crypto.SecretKey
}

type PrevPackets []header.TCP

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func (pps PrevPackets) Client(ctx context.Context, conn net.Conn) error {
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

func (pps PrevPackets) Server(ctx context.Context, conn net.Conn) error {
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
