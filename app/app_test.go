package app_test

import (
	"net/netip"
	"time"

	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/crypto"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

var (
	caddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{
		// 172, 25, 32, 1,
		172, 24, 128, 1,
	}), 19986)

	saddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{
		// 172, 25, 38, 4,
		172, 24, 131, 26,
	}), 8080)

	ht = time.Hour
)

type tkClient struct{}

func (c *tkClient) Token() (tk []byte, key crypto.Key, err error) {
	return []byte("hello"), crypto.Key{1: 1}, nil
}

type tkServer struct{}

func (c *tkServer) Valid(tk []byte) (key crypto.Key, err error) {
	if string(tk) == "hello" {
		return crypto.Key{1: 1}, nil
	}
	return crypto.Key{}, errors.Errorf("invalid token")
}

var pps = config.PrevPackets{
	header.TCP("hello"),
	header.TCP("world"),
}

/*
	func Test_Handshake(t *testing.T) {
		var (
			caddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, 0, 1}), 19986)
			saddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte{1, 1, 1, 1}), 8080)
		)
		rawc, raws := test.NewMockRaw(
			t, header.TCPProtocolNumber,
			caddr, saddr,
			test.ValidAddr, test.ValidChecksum,
		)

		// server
		go func() {
			l := test.NewMockListener(t, raws)
			cfg := &server.Config{
				Sconn: sconn.Server{
					BaseConfig: sconn.BaseConfig{
						PrevPackets:      pps,
						HandShakeTimeout: ht,
					},
					SwapKey: &sconn.TokenServer{Valider: &tkServer{}},
				},
				MTU:                 1536,
				TCPHandshakeTimeout: ht,
				InitCfgTimeout:      ht,
				ProxyerIdeleTimeout: ht,
			}

			server.ListenAndServe(context.Background(), l, cfg)
		}()
		time.Sleep(time.Second)

		{ // client
			cfg := &client.Config{
				Sconn: sconn.Client{
					BaseConfig: sconn.BaseConfig{
						PrevPackets:      pps,
						HandShakeTimeout: ht,
					},
					SwapKey: &sconn.TokenClient{Tokener: &tkClient{}},
				},
				MTU: 1536,
			}

			ctx := context.Background()

			c, err := client.NewClient(ctx, rawc, cfg)
			require.NoError(t, err)
			defer c.Close()
		}
	}
*/
