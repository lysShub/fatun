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
	"github.com/lysShub/itun/segment"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Conn struct {
	raw *itun.RawConn

	fake *fake.FakeTCP

	crypter *crypto.TCPCrypt
}

func (s *Conn) RecvSeg(ctx context.Context, seg *segment.Segment) (err error) {
	p := seg.Packet()

	err = s.raw.ReadCtx(ctx, p)
	if err != nil {
		return err
	}

	if s.crypter != nil {
		err = s.crypter.Decrypt(p)
		if err != nil {
			return err
		}
	}
	s.fake.AttachRecv(p)

	tcphdr := header.TCP(p.Data())
	p.SetHead(p.Head() + int(tcphdr.DataOffset())) // remove tcp header

	return nil
}

func (s *Conn) SendSeg(ctx context.Context, seg *segment.Segment) (err error) {
	p := seg.Packet()

	s.fake.AttachSend(p)

	if s.crypter != nil {
		s.crypter.Encrypt(p)
	}

	return s.raw.WriteCtx(ctx, p)
}

func (s *Conn) Raw() *itun.RawConn {
	return s.raw
}

func (s *Conn) Close() error {
	return s.raw.Close()
}

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func Accept(parentCtx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (s *Conn) {
	ctx := cctx.WithTimeout(parentCtx, time.Second*15) // todo: from cfg
	defer ctx.Cancel(nil)

	s = accept(ctx, raw, cfg)
	if err := ctx.Err(); err != nil {
		parentCtx.Cancel(err)
		return nil
	}
	return s
}

func Connect(parentCtx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (s *Conn) {
	ctx := cctx.WithTimeout(parentCtx, time.Second*15)
	defer ctx.Cancel(nil)

	s = connect(ctx, raw, cfg)
	if err := ctx.Err(); err != nil {
		parentCtx.Cancel(err)
	}
	return s
}

func accept(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (conn *Conn) {
	conn = &Conn{raw: raw}

	us := newUserStack(ctx, raw)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	tcpAcceptCtx := cctx.WithTimeout(ctx, time.Second*5)
	tcp := us.Accept(tcpAcceptCtx)
	if err := tcpAcceptCtx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close()

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

	seq, ack := us.SeqAck()
	conn.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return conn
}

func connect(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (conn *Conn) {
	conn = &Conn{raw: raw}

	us := newUserStack(ctx, raw)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	tcp := us.Connect(ctx)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}
	defer tcp.Close()

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

	seq, ack := us.SeqAck()
	conn.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return conn
}
