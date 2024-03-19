package crypto_test

import (
	"context"
	"io"
	"testing"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/test"
	"github.com/stretchr/testify/require"
)

func UnicomStackAndRaw(t *testing.T, s ustack.Ustack, raw *itun.RawConn, pseudoSum1 uint16) {
	c, err := crypto.NewTCP(crypto.Key{0: 1}, pseudoSum1)
	require.NoError(t, err)

	go func() {
		mtu := raw.MTU()
		var ip = rsocket.NewPacket(0, mtu)

		for {
			ip.Sets(0, mtu)
			s.Outbound(context.Background(), ip)
			if ip.Len() == 0 {
				return
			}

			ip.SetHead(0)
			test.ValidIP(t, ip.Data())

			c.EncryptRaw(ip)

			test.ValidIP(t, ip.Data())

			_, err := raw.Write(ip.Data())
			require.NoError(t, err)
		}
	}()
	go func() {
		mtu := raw.MTU()
		var tcp = rsocket.NewPacket(0, mtu)

		for {
			tcp.Sets(0, mtu)
			err := raw.ReadCtx(context.Background(), tcp)
			if errors.Is(err, io.EOF) {
				return
			}
			require.NoError(t, err)

			tcp.SetHead(0)
			test.ValidIP(t, tcp.Data())

			err = c.DecryptRaw(tcp)
			require.NoError(t, err)

			test.ValidIP(t, tcp.Data())

			s.Inbound(tcp)
		}
	}()
}
