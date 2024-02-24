package sconn

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lysShub/itun/cctx"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type PrevPackets []header.TCP

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

type BaseConfig struct {
	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets PrevPackets //todo: support mutiple data set

	HandShakeTimeout time.Duration
}

type Client struct {
	BaseConfig

	// swap secret key
	SwapKey SecretKeyClient
}

type Server struct {
	BaseConfig

	// swap secret key
	SwapKey SecretKeyServer
}

func (pps PrevPackets) Client(ctx cctx.CancelCtx, conn net.Conn) {
	for i := 0; i < len(pps); i++ {
		if i%2 == 0 {
			_, err := conn.Write(pps[i])
			if err != nil {
				ctx.Cancel(err)
				return
			}
		} else {
			var b = make([]byte, len(pps[i]))

			if _, err := io.ReadFull(conn, b); err != nil {
				ctx.Cancel(err)
				return
			}
			if !bytes.Equal(b, pps[i]) {
				ctx.Cancel(ErrPrevPacketInvalid(i))
				return
			}
		}
	}
}

func (pps PrevPackets) Server(ctx cctx.CancelCtx, conn net.Conn) {
	for i := 0; i < len(pps); i++ {
		if i%2 == 0 {
			var b = make([]byte, len(pps[i]))

			if _, err := io.ReadFull(conn, b); err != nil {
				ctx.Cancel(err)
				return
			}
			if !bytes.Equal(b, pps[i]) {
				ctx.Cancel(ErrPrevPacketInvalid(i))
				return
			}
		} else {
			_, err := conn.Write(pps[i])
			if err != nil {
				ctx.Cancel(err)
				return
			}
		}
	}
}
