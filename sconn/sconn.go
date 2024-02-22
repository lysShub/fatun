package sconn

import (
	"context"
	"fmt"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/fake"
	"github.com/lysShub/itun/sconn/crypto"
	"github.com/lysShub/itun/segment"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Conn struct {
	raw *itun.RawConn

	state state // todo: atomic

	fake *fake.FakeTCP

	crypter *crypto.TCPCrypt

	tinyCnt    uint8
	tinyCntErr error

	psosum1 uint16
}

type state uint8

const (
	_ state = iota
	handshake

	// if sconn crypter!=nil, next stage packet will encrypto
	transport
	closeing
)

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
		return err
	}
	oldH, oldN := seg.Head(), seg.Len()

	err = s.raw.ReadCtx(ctx, seg.Packet())
	if err != nil {
		return err
	}

	// recved impostor/wrong packet
	n := len(header.TCP(seg.Data()).Payload())
	if n < segment.HdrSize+header.UDPMinimumSize {
		s.tinyCnt++
		s.tinyCntErr = ErrManyInvalidSizeSegment(n)
		seg.Sets(oldH, oldN)
		return s.RecvSeg(ctx, seg)
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
		s.crypter.EncryptChecksum(p, s.psosum1)
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
