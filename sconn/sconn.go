package sconn

import (
	"context"
	"fmt"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/sconn/crypto"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"github.com/stretchr/testify/require"
)

// security datagram conn (fake tcp connn)
type Conn struct {
	raw *itun.RawConn
	id  string

	fake *faketcp.FakeTCP

	crypter *crypto.TCP

	tinyCnt    uint8
	tinyCntErr error
}

const tinyCntLimit = 4 // todo: to config

type ErrManyInvalidSizePacket int

func (e ErrManyInvalidSizePacket) Error() string {
	return fmt.Sprintf("recved many invalid size(%d) packet", int(e))
}

type ErrManyDecryptFailPacket string

func (e ErrManyDecryptFailPacket) Error() string {
	return fmt.Sprintf("recved many decrypt fail packet, %s", string(e))
}

func (s *Conn) RecvSeg(ctx context.Context, b *relraw.Packet) (id session.SessID, err error) {
	if s.tinyCnt > tinyCntLimit {
		return 0, s.tinyCntErr
	}
	oldH, oldN := b.Head(), b.Len()

	err = s.raw.ReadCtx(ctx, b)
	if err != nil {
		return 0, err
	}

	if s.crypter != nil {
		s.tinyCnt++
		err = s.crypter.Decrypt(b)

		// recved impostor/wrong packet
		if err != nil {
			b.Sets(oldH, oldN)
			s.tinyCntErr = ErrManyDecryptFailPacket(err.Error())
			return s.RecvSeg(ctx, b)
		}
	}

	s.fake.RecvStrip(b)

	if b.Len() <= session.IDSize {
		s.tinyCnt++
		s.tinyCntErr = ErrManyInvalidSizePacket(b.Len())
		b.Sets(oldH, oldN)
		return s.RecvSeg(ctx, b)
	}

	s.tinyCnt = 0
	return session.GetID(b), nil
}

func (s *Conn) SendSeg(ctx context.Context, b *relraw.Packet, id session.SessID) (err error) {
	session.SetID(b, id)

	s.fake.SendAttach(b)

	if s.crypter != nil {
		s.crypter.Encrypt(b)

		if debug.Debug() {
			tp := b.Copy()
			err := s.crypter.Decrypt(tp)
			require.NoError(test.T(), err)
		}
	}

	// todo: raw not calc tcp checksum
	return s.raw.WriteCtx(ctx, b)
}

func (s *Conn) Raw() *itun.RawConn {
	return s.raw
}

func (s *Conn) Close() error {
	return s.raw.Close()
}
