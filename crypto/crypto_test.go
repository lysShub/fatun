package crypto_test

import (
	"context"
	"io"
	"testing"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/ustack"

	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/stretchr/testify/require"
)

func UnicomStackAndRaw(t *testing.T, s ustack.Ustack, raw *itun.RawConn, pseudoSum1 uint16) {
	c, err := crypto.NewTCP(crypto.Key{0: 1}, pseudoSum1)
	require.NoError(t, err)

	go func() {
		mtu := raw.MTU()
		var ip = packet.Make(0, mtu)

		for {
			ip.Sets(0, mtu)
			s.Outbound(context.Background(), ip)
			if ip.Data() == 0 {
				return
			}

			ip.SetHead(0)
			test.ValidIP(t, ip.Bytes())

			c.EncryptRaw(ip)

			test.ValidIP(t, ip.Bytes())

			err := raw.Write(context.Background(), ip)
			require.NoError(t, err)
		}
	}()
	go func() {
		mtu := raw.MTU()
		var tcp = packet.Make(0, mtu)

		for {
			tcp.Sets(0, mtu)
			err := raw.Read(context.Background(), tcp)
			if errors.Is(err, io.EOF) {
				return
			}
			require.NoError(t, err)

			tcp.SetHead(0)
			test.ValidIP(t, tcp.Bytes())

			err = c.DecryptRaw(tcp)
			require.NoError(t, err)

			test.ValidIP(t, tcp.Bytes())

			s.Inbound(tcp)
		}
	}()
}
