package sconn

import (
	"errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn/crypto"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/faketcp"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Accept(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Server) (conn *Conn) {
	tcp, err := ustack.AcceptNoFinTCP(ctx, raw, cfg.HandShakeTimeout)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close() // todo: aceept error

	conn = &Conn{
		raw:   raw,
		id:    "server",
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

	// NO-FIN close
	if err := tcp.Close(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	seq, ack := tcp.SeqAck()
	conn.fake = faketcp.NewFakeTCP(
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

func Connect(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Client) (conn *Conn) {
	tcp, err := ustack.ConnectNoFinTCP(ctx, raw, cfg.HandShakeTimeout)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close()

	conn = &Conn{raw: raw, id: "clinet", state: handshake}

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

	// NO-FIN close
	if err := tcp.Close(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	seq, ack := tcp.SeqAck()
	conn.fake = faketcp.NewFakeTCP(
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
