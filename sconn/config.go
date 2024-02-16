package sconn

import (
	"bytes"
	"context"
	"io"
	"net"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn/crypto"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type PrevPackets []header.TCP

type Config struct {
	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets PrevPackets //todo: support mutiple data set

	// default false
	Crypto bool

	// swap secret key, not nil if Crypto set
	SwapKey SwapSercetKey
}

type SwapSercetKey interface {
	SendKey(ctx context.Context, tcp net.Conn) ([crypto.Bytes]byte, error)
	RecvKey(ctx context.Context, tcp net.Conn) ([crypto.Bytes]byte, error)
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
