package sconn

import (
	"context"
	"errors"
	"fmt"
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
	*itun.RawConn

	fake *fake.FakeTCP

	crypto  bool
	crypter *crypto.TCPCrypt
}

// todo: return reserved
func (s *Conn) Read(b []byte) (seg segment.Segment, err error) {
	for len(seg) > 0 {
		n, err := s.RawConn.Read(b)
		if err != nil {
			return nil, err
		}

		iphdr := header.IPv4(b[:n]) // todo: ipv6
		tcphdr := header.TCP(iphdr.Payload())
		if s.crypto {
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

func (s *Conn) Write(b segment.Segment, reserved int) (err error) {
	tcp, empty := s.fake.Send(b, reserved)

	ptr := uintptr(unsafe.Pointer(&tcp[0]))
	if s.crypto {
		tcp = s.crypter.Encrypt(tcp)
	}

	if ptr == uintptr(unsafe.Pointer(&tcp[0])) {
		return s.RawConn.WriteReservedIPHeader(tcp, empty)
	} else {
		_, err = s.RawConn.Write(b)
		return err
	}
}

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func Accept(c context.Context, raw *itun.RawConn, cfg *Config) (s *Conn, err error) {
	ctx := cctx.WithTimeout(c, time.Second*30) // todo: from cfg
	defer ctx.Cancel(nil)

	var conn = &Conn{
		RawConn: raw,
		crypto:  cfg.Crypto,
	}
	us := newUserStack(ctx, raw)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tcp := us.Accept(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer tcp.Close()

	// previous packets
	cfg.PrevPackets.Server(ctx, tcp)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// swap secret key
	if cfg.Crypto && cfg.SwapKey != nil {
		if key, err := cfg.SwapKey.RecvKey(ctx, tcp); err != nil {
			return nil, errors.Join(ctx.Err(), err)
		} else {
			conn.crypter, err = crypto.NewTCPCrypt(key)
			if err != nil {
				return nil, errors.Join(ctx.Err(), err)
			}
		}
	}

	seq, ack := us.SeqAck()
	s.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return s, nil
}

func Connect(ctx context.Context, raw *itun.RawConn, cfg *Config) (s *Conn, err error) {
	cc := cctx.WithTimeout(ctx, time.Second*30)
	defer cc.Cancel(nil)

	var conn = &Conn{
		RawConn: raw,
		crypto:  cfg.Crypto,
	}
	us := newUserStack(cc, raw)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tcp := us.Connect(cctx.WithTimeout(cc, time.Second*5))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer tcp.Close()

	// previous packets
	cfg.PrevPackets.Client(cc, tcp)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// swap secret key
	if cfg.Crypto && cfg.SwapKey != nil {
		if key, err := cfg.SwapKey.SendKey(ctx, tcp); err != nil {
			return nil, errors.Join(ctx.Err(), err)
		} else {
			conn.crypter, err = crypto.NewTCPCrypt(key)
			if err != nil {
				return nil, errors.Join(ctx.Err(), err)
			}
		}
	}

	seq, ack := us.SeqAck()
	s.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return s, nil
}
