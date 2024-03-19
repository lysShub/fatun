//go:build linux
// +build linux

package sender

import (
	"context"
	"net/netip"
	"time"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/tcp"
	"github.com/lysShub/rsocket/test"
	"github.com/lysShub/rsocket/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
)

type sender struct {
	raw     rsocket.RawConn
	ipstack *rsocket.IPStack
	start   time.Time
}

func newSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (*sender, error) {
	ipstack, err := rsocket.NewIPStack(
		loc.Addr(), dst.Addr(),
		tcpip.TransportProtocolNumber(proto),
	)
	if err != nil {
		return nil, err
	}

	switch proto {
	case itun.TCP:
		tcp, err := tcp.Connect(
			loc, dst,
			rsocket.UsedPort(), // PortAdapter bind the port
		)
		if err != nil {
			return nil, err
		}

		return &sender{
			raw:     tcp,
			ipstack: ipstack,
			start:   time.Now(),
		}, nil
	default:
		return nil, errors.New("not support")
	}
}

func (s *sender) Send(pkt *rsocket.Packet) error {
	s.ipstack.AttachOutbound(pkt)
	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Data())
	}

	_, err := s.raw.Write(pkt.Data())
	return err
}

func (s *sender) Recv(ctx context.Context, pkt *rsocket.Packet) error {
	err := s.raw.ReadCtx(ctx, pkt)
	return err
}

func (s *sender) Close() error {
	return s.raw.Close()
}
