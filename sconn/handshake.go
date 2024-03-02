package sconn

import (
	"errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn/crypto"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/itun/ustack/link/nofin"
	"github.com/lysShub/relraw/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Accept(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (conn *Conn) {
	link := nofin.New(4, uint32(raw.MTU()))
	tcp, err := ustack.AcceptTCP(ctx, raw, link, cfg.HandShakeTimeout)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close() // todo: aceept error

	conn = &Conn{raw: raw, id: "server"}

	// previous packets
	cfg.PrevPackets.Server(ctx, tcp)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	pseudoSum1 := header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		conn.raw.LocalAddr().Addr, conn.raw.RemoteAddr().Addr,
		0,
	)

	// swap secret key
	if key, err := cfg.SwapKey.SecretKey(ctx, tcp); err != nil {
		ctx.Cancel(errors.Join(ctx.Err(), err))
		return nil
	} else {
		if key != [crypto.Bytes]byte{} {
			conn.crypter, err = crypto.NewTCP(key, pseudoSum1)
			if err != nil {
				ctx.Cancel(errors.Join(ctx.Err(), err))
				return nil
			}
		}
	}

	// NO-FIN close
	if err := tcp.Close(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	// crypto tcp packet will re-calc checksum always
	var psum1 *uint16
	if conn.crypter == nil {
		psum1 = &pseudoSum1
	}
	if debug.Debug() {
		psum1 = &pseudoSum1
	}
	seq, ack := link.SeqAck()
	conn.fake = faketcp.NewFakeTCP(
		raw.LocalAddr().Port,
		raw.RemoteAddr().Port,
		seq, ack, psum1,
	)
	return conn
}

func Connect(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (conn *Conn) {
	link := nofin.New(4, uint32(raw.MTU()))
	tcp, err := ustack.ConnectTCP(ctx, raw, link, cfg.HandShakeTimeout)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close()

	conn = &Conn{raw: raw, id: "client"}

	// previous packets
	cfg.PrevPackets.Client(ctx, tcp)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	pseudoSum1 := header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		conn.raw.LocalAddr().Addr, conn.raw.RemoteAddr().Addr,
		0,
	)

	// swap secret key
	if key, err := cfg.SwapKey.SecretKey(ctx, tcp); err != nil {
		ctx.Cancel(err)
		return nil
	} else {
		if key != (Key{}) {
			conn.crypter, err = crypto.NewTCP(key, pseudoSum1)
			if err != nil {
				ctx.Cancel(err)
				return nil
			}
		}
	}

	// NO-FIN close
	if err := tcp.Close(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	// crypto tcp packet will re-calc checksum always
	var psum1 *uint16
	if conn.crypter == nil {
		psum1 = &pseudoSum1
	}
	if debug.Debug() {
		psum1 = &pseudoSum1
	}
	seq, ack := link.SeqAck()
	conn.fake = faketcp.NewFakeTCP(
		raw.LocalAddr().Port,
		raw.RemoteAddr().Port,
		seq, ack, psum1,
	)
	return conn
}
