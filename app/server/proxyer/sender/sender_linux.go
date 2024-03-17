//go:build linux
// +build linux

package sender

import (
	"context"
	"net/netip"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
)

type sender struct {
	raw     relraw.RawConn
	ipstack *relraw.IPStack
}

func newSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (*sender, error) {
	ipstack, err := relraw.NewIPStack(
		loc.Addr(), dst.Addr(),
		tcpip.TransportProtocolNumber(proto),
	)
	if err != nil {
		return nil, err
	}

	switch proto {
	case itun.TCP:
		tcp, err := bpf.Connect(
			loc, dst,
			relraw.UsedPort(), // PortAdapter bind the port
		)
		if err != nil {
			return nil, err
		}

		return &sender{raw: tcp, ipstack: ipstack}, nil
	default:
		return nil, errors.New("not support")
	}
}

func (s *sender) Send(pkt *relraw.Packet) error {
	s.ipstack.AttachOutbound(pkt)
	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Data())
	}

	_, err := s.raw.Write(pkt.Data())
	return err
}

func (s *sender) Recv(ctx context.Context, pkt *relraw.Packet) error {
	err := s.raw.ReadCtx(ctx, pkt)
	return err
}

func (s *sender) Close() error {
	return s.raw.Close()
}
