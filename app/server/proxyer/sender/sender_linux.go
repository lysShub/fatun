//go:build linux
// +build linux

package sender

import (
	"context"
	"net/netip"
	"time"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
	"github.com/lysShub/sockit/conn"
	"github.com/lysShub/sockit/helper/ipstack"
	"github.com/lysShub/sockit/packet"

	"github.com/lysShub/sockit/conn/tcp"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type sender struct {
	raw        conn.RawConn
	ipstack    *ipstack.IPStack
	start      time.Time
	pseudoSum1 uint16
}

func newSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (*sender, error) {
	ipstack, err := ipstack.New(
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
			conn.UsedPort(), // PortAdapter bind the port
		)
		if err != nil {
			return nil, err
		}

		// tcp, err = test.WrapPcap(tcp, "sender.pcap")
		// if err != nil {
		// 	panic(err)
		// }

		pseudoSum1 := header.PseudoHeaderChecksum(
			header.TCPProtocolNumber,
			tcpip.AddrFromSlice(loc.Addr().AsSlice()),
			tcpip.AddrFromSlice(dst.Addr().AsSlice()),
			0,
		)
		return &sender{
			raw:        tcp,
			ipstack:    ipstack,
			start:      time.Now(),
			pseudoSum1: pseudoSum1,
		}, nil
	default:
		return nil, errors.New("not support")
	}
}

func (s *sender) Send(pkt *packet.Packet) error {

	// todo: optimize
	tcp := header.TCP(pkt.Bytes())
	tcp.SetChecksum(0)
	sum := checksum.Combine(s.pseudoSum1, uint16(len(tcp)))
	sum = checksum.Checksum(tcp, sum)
	tcp.SetChecksum(^sum)

	if debug.Debug() {
		test.ValidTCP(test.T(), pkt.Bytes(), s.pseudoSum1)
	}

	err := s.raw.Write(context.Background(), pkt)
	return err
}

func (s *sender) Recv(ctx context.Context, pkt *packet.Packet) error {
	err := s.raw.Read(ctx, pkt)
	return err
}

func (s *sender) Close() error {
	return s.raw.Close()
}
