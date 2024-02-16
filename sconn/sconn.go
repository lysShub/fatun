package sconn

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"
	"unsafe"

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

// todo: return offset
func (s *Conn) RecvSeg(b []byte) (seg segment.Segment, err error) {
	for len(seg) > 0 {
		n, err := s.raw.Read(b)
		if err != nil {
			return nil, err
		}

		iphdr := header.IPv4(b[:n]) // todo: ipv6
		tcphdr := header.TCP(iphdr.Payload())
		if s.crypter != nil {
			tcphdr, err = s.crypter.Decrypt(tcphdr)
			if err != nil {
				return nil, err
			}
		}
		s.fake.Recv(tcphdr)
		seg = segment.Segment(tcphdr.Payload())
	}

	return seg, err
}

func (s *Conn) SendSeg(seg segment.Segment, reserved int) (err error) {
	tcp, empty := s.fake.Send(seg, reserved)

	ptr := uintptr(unsafe.Pointer(&tcp[0]))
	if s.crypter != nil {
		tcp = s.crypter.Encrypt(tcp)
	}

	if ptr == uintptr(unsafe.Pointer(&tcp[0])) {
		return s.raw.WriteReserved(tcp, empty)
	} else {
		_, err = s.raw.Write(seg)
		return err
	}
}

func (s *Conn) Raw() *itun.RawConn {
	return s.raw
}

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func Accept(parentCtx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (s *Conn) {
	ctx := cctx.WithTimeout(parentCtx, time.Second*30) // todo: from cfg
	defer ctx.Cancel(nil)

	s = accept(ctx, raw, cfg)
	if err := ctx.Err(); err != nil {
		parentCtx.Cancel(err)
		return nil
	}
	return s
}

func Connect(parentCtx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (s *Conn) {
	ctx := cctx.WithTimeout(parentCtx, time.Second*30)
	defer ctx.Cancel(nil)

	s = connect(ctx, raw, cfg)
	if err := ctx.Err(); err != nil {
		parentCtx.Cancel(err)
	}
	return s
}

func accept(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (s *Conn) {
	var conn = &Conn{raw: raw}

	us := newUserStack(ctx, raw)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	tcp := us.Accept(ctx)
	if err := ctx.Err(); err != nil {
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
	if cfg.SwapKey != nil {
		key, err := cfg.SwapKey.RecvKey(ctx, tcp)
		if err != nil && !tcpClosed(tcp) {
			ctx.Cancel(errors.Join(ctx.Err(), err))
			return nil
		}

		conn.crypter, err = crypto.NewTCPCrypt(key)
		if err != nil {
			ctx.Cancel(errors.Join(ctx.Err(), err))
			return nil
		}
	}

	seq, ack := us.SeqAck()
	s.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return s
}

func connect(ctx cctx.CancelCtx, raw *itun.RawConn, cfg *Config) (s *Conn) {
	var conn = &Conn{raw: raw}
	us := newUserStack(ctx, raw)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	tcp := us.Connect(cctx.WithTimeout(ctx, time.Second*5))
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
	if cfg.SwapKey != nil {
		key, err := cfg.SwapKey.SendKey(ctx, tcp)
		if err != nil && !tcpClosed(tcp) {
			ctx.Cancel(errors.Join(ctx.Err(), err))
			return nil
		}

		conn.crypter, err = crypto.NewTCPCrypt(key)
		if err != nil {
			ctx.Cancel(errors.Join(ctx.Err(), err))
			return nil
		}
	}

	seq, ack := us.SeqAck()
	s.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return s
}

func tcpClosed(tcp net.Conn) bool {
	n, err := tcp.Read(make([]byte, 0))
	return n == 0 && errors.Is(err, os.ErrClosed)
}
