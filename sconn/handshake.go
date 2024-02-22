package sconn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/fake"
	"github.com/lysShub/itun/sconn/crypto"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func Accept(parentCtx cctx.CancelCtx, tcpHandshakeTimeout time.Duration, raw *itun.RawConn, cfg *Server) (s *Conn) {
	ctx := cctx.WithTimeout(parentCtx, tcpHandshakeTimeout)
	defer ctx.Cancel(nil)

	s = accept(ctx, raw, cfg, tcpHandshakeTimeout)
	if err := ctx.Err(); err != nil {
		parentCtx.Cancel(err)
		return nil
	}
	return s
}

func Connect(parentCtx cctx.CancelCtx, raw *itun.RawConn, cfg *Client) (s *Conn) {
	ctx := cctx.WithTimeout(parentCtx, time.Second*15)
	defer ctx.Cancel(nil)

	s = connect(ctx, raw, cfg)
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		parentCtx.Cancel(err)
	}
	return s
}

func accept(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Server, tcpHandshakeTimeout time.Duration) (conn *Conn) {
	tcp := AcceptTCP(ctx, raw, tcpHandshakeTimeout)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close() // todo: aceept error

	conn = &Conn{
		raw:   raw,
		state: handshake,
	}

	// previous packets
	cfg.PrevPackets.Server(ctx, tcp)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	// swap secret key
	if key, err := cfg.SwapKey.SecretKey(ctx, tcp); err != nil {
		ctx.Cancel(errors.Join(ctx.Err(), err))
		return nil
	} else {
		if key != [crypto.Bytes]byte{} {
			conn.crypter, err = crypto.NewTCPCrypt(key)
			if err != nil {
				ctx.Cancel(errors.Join(ctx.Err(), err))
				return nil
			}
		}
	}

	if err := tcp.Close(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	seq, ack := tcp.SeqAck()
	conn.fake = fake.NewFakeTCP(
		raw.LocalAddr().Port,
		raw.RemoteAddr().Port,
		seq, ack, conn.crypter == nil,
	)
	conn.state = transport
	conn.psosum1 = header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		conn.raw.LocalAddr().Addr, conn.raw.RemoteAddr().Addr,
		0,
	)
	return conn
}

func connect(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Client) (conn *Conn) {
	tcp := ConnectTCP(ctx, raw)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close()

	conn = &Conn{raw: raw, state: handshake}

	// previous packets
	cfg.PrevPackets.Client(ctx, tcp)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	// swap secret key
	if key, err := cfg.SwapKey.SecretKey(ctx, tcp); err != nil {
		ctx.Cancel(err)
		return nil
	} else {
		if key != (Key{}) {
			conn.crypter, err = crypto.NewTCPCrypt(key)
			if err != nil {
				ctx.Cancel(err)
				return nil
			}
		}
	}

	if err := tcp.Close(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	seq, ack := tcp.SeqAck()
	conn.fake = fake.NewFakeTCP(
		raw.LocalAddr().Port,
		raw.RemoteAddr().Port,
		seq, ack, conn.crypter == nil,
	)
	conn.state = transport
	conn.psosum1 = header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		conn.raw.LocalAddr().Addr, conn.raw.RemoteAddr().Addr,
		0,
	)
	return conn
}
