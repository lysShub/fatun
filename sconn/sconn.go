package sconn

import (
	"context"
	"fmt"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/sconn/crypto"
	"github.com/lysShub/itun/segment"
	"github.com/lysShub/itun/ustack/faketcp"
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

type ErrManyInvalidSizeSegment int

func (e ErrManyInvalidSizeSegment) Error() string {
	return fmt.Sprintf("recved many invalid size(%d) segment", int(e))
}

type ErrManyDecryptFailSegment string

func (e ErrManyDecryptFailSegment) Error() string {
	return fmt.Sprintf("recved many decrypt fail segment, %s", string(e))
}

func (s *Conn) RecvSeg(ctx context.Context, seg *segment.Segment) (err error) {
	if s.tinyCnt > tinyCntLimit {
		return s.tinyCntErr
	}
	oldH, oldN := seg.Head(), seg.Len()

	err = s.raw.ReadCtx(ctx, seg.Packet())
	if err != nil {
		return err
	}

	if s.crypter != nil {
		s.tinyCnt++
		err = s.crypter.Decrypt(seg.Packet())

		// recved impostor/wrong packet
		if err != nil {
			seg.Sets(oldH, oldN)
			s.tinyCntErr = ErrManyDecryptFailSegment(err.Error())
			return s.RecvSeg(ctx, seg)
		}
	}

	s.fake.RecvStrip(seg.Packet())

	s.tinyCnt = 0
	return nil
}

func (s *Conn) SendSeg(ctx context.Context, seg *segment.Segment) (err error) {
	p := seg.Packet()

	s.fake.SendAttach(p)

	if s.crypter != nil {
		s.crypter.Encrypt(p)

		if debug.Debug() {
			tp := p.Copy()
			err := s.crypter.Decrypt(tp)
			require.NoError(test.T(), err)
		}
	}

	// todo: raw not calc tcp checksum
	return s.raw.WriteCtx(ctx, p)
}

func (s *Conn) Raw() *itun.RawConn {
	return s.raw
}

func (s *Conn) Close() error {
	return s.raw.Close()
}
