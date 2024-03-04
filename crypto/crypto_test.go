package crypto_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
)

func UnicomStackAndRaw(t *testing.T, s *ustack.Ustack, raw *itun.RawConn, pseudoSum1 uint16) {
	c, err := crypto.NewTCP(crypto.Key{0: 1}, pseudoSum1)
	require.NoError(t, err)

	go func() {
		mtu := raw.MTU()
		var p = relraw.NewPacket(0, mtu)

		for {
			p.Sets(0, mtu)
			s.Outbound(context.Background(), p)
			if p.Len() == 0 {
				return
			}

			test.ValidIP(t, p.Data())

			c.EncryptRaw(p)

			test.ValidIP(t, p.Data())

			_, err := raw.Write(p.Data())
			require.NoError(t, err)
		}
	}()
	go func() {
		mtu := raw.MTU()
		var p = relraw.NewPacket(0, mtu)

		for {
			p.Sets(0, mtu)
			err := raw.ReadCtx(context.Background(), p)
			if errors.Is(err, io.EOF) {
				return
			}
			require.NoError(t, err)

			p.Sets(0, p.Head()+p.Len())
			test.ValidIP(t, p.Data())

			err = c.DecryptRaw(p)
			require.NoError(t, err)

			test.ValidIP(t, p.Data())

			s.Inbound(p)
		}
	}()
}
