package sconn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"unsafe"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/fake"
	"github.com/lysShub/itun/sconn/crypto"
	"github.com/lysShub/itun/segment"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets []header.TCP //todo: support mutiple data set

	// default false
	Crypto bool

	// swap secret key, not nil if Crypto set
	SwapKey SwapSercetKey
}

type SwapSercetKey interface {
	SendKey(ctx context.Context, tcp net.Conn) ([crypto.Bytes]byte, error)
	RecvKey(ctx context.Context, tcp net.Conn) ([crypto.Bytes]byte, error)
}

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

func Accept(ctx context.Context, raw *itun.RawConn, cfg *Config) (s *Conn, err error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var c = &Conn{
		RawConn: raw,
		crypto:  cfg.Crypto,
	}

	var tcp net.Conn
	us, err := newUserStack(ctx, cancel, raw)
	if err != nil {
		return nil, err
	} else if tcp, err = us.Accept(ctx); err != nil {
		return nil, err
	}
	defer tcp.Close()

	// previous packets
	for i := 0; i < len(cfg.PrevPackets); i++ {
		if i%2 == 0 {
			var b = make([]byte, len(cfg.PrevPackets[i]))

			if _, err := io.ReadFull(tcp, b); err != nil {
				return nil, err
			}
			if !bytes.Equal(b, cfg.PrevPackets[i]) {
				return nil, ErrPrevPacketInvalid(i)
			}
		} else {
			_, err := tcp.Write(cfg.PrevPackets[i])
			if err != nil {
				return nil, err
			}
		}
	}

	// get secret key
	if cfg.Crypto && cfg.SwapKey != nil {
		if key, err := cfg.SwapKey.RecvKey(ctx, tcp); err != nil {
			return nil, err
		} else {
			c.crypter, err = crypto.NewTCPCrypt(key)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if err := tcp.Close(); err != nil {
			return nil, err
		}
	}

	seq, ack := us.SeqAck()
	s.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return s, nil
}

func Connect(ctx context.Context, raw *itun.RawConn, cfg *Config) (s *Conn, err error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var c = &Conn{
		RawConn: raw,
		crypto:  cfg.Crypto,
	}

	var tcp net.Conn
	us, err := newUserStack(ctx, cancel, raw)
	if err != nil {
		return nil, err
	} else if tcp, err = us.Connect(ctx); err != nil {
		return nil, err
	}

	// previous packets
	for i := 0; i < len(cfg.PrevPackets); i++ {
		if i%2 == 0 {
			_, err := tcp.Write(cfg.PrevPackets[i])
			if err != nil {
				return nil, err
			}
		} else {
			var b = make([]byte, len(cfg.PrevPackets[i]))

			if _, err := io.ReadFull(tcp, b); err != nil {
				return nil, err
			}
			if !bytes.Equal(b, cfg.PrevPackets[i]) {
				return nil, ErrPrevPacketInvalid(i)
			}
		}
	}
	defer tcp.Close()

	// send secret key
	if cfg.Crypto && cfg.SwapKey != nil {
		if key, err := cfg.SwapKey.SendKey(ctx, tcp); err != nil {
			return nil, err
		} else {
			c.crypter, err = crypto.NewTCPCrypt(key)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if err := tcp.Close(); err != nil {
			return nil, err
		}
	}

	seq, ack := us.SeqAck()
	s.fake = fake.NewFakeTCP(raw.LocalAddr().Port, raw.RemoteAddr().Port, seq, ack)
	return s, nil
}
