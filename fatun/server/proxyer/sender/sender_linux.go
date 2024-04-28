//go:build linux
// +build linux

package sender

import (
	"context"
	"net/netip"

	"github.com/pkg/errors"

	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/conn"
	"github.com/lysShub/sockit/packet"

	"github.com/lysShub/sockit/conn/tcp"
	udp "github.com/lysShub/sockit/conn/udp/raw"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type sender struct {
	raw        conn.RawConn
	proto      tcpip.TransportProtocolNumber
	pseudoSum1 uint16
}

func newSender(local netip.AddrPort, proto tcpip.TransportProtocolNumber, remote netip.AddrPort) (*sender, error) {
	pseudoSum1 := header.PseudoHeaderChecksum(
		proto,
		tcpip.AddrFromSlice(local.Addr().AsSlice()),
		tcpip.AddrFromSlice(remote.Addr().AsSlice()),
		0,
	)

	var s = &sender{proto: proto, pseudoSum1: pseudoSum1}

	var err error
	switch proto {
	case header.TCPProtocolNumber:
		if s.raw, err = tcp.Connect(
			local, remote,
			conn.UsedPort(), // PortAdapter bind the port
		); err != nil {
			return nil, err
		}

		// s.raw, err = test.WrapPcap(s.raw, "sender.pcap")
		// if err != nil {
		// 	panic(err)
		// }
	case header.UDPProtocolNumber:
		if s.raw, err = udp.Connect(
			local, remote,
			conn.UsedPort(),
		); err != nil {
			return nil, err
		}
	default:
		return nil, errors.WithStack(session.ErrNotSupportProto(proto))
	}

	return s, nil
}

func (s *sender) Send(pkt *packet.Packet) error {

	// re-calc checksum, src addr-port changed
	// todo: optimize
	var t header.Transport
	if s.proto == header.TCPProtocolNumber {
		t = header.TCP(pkt.Bytes())
	} else {
		t = header.UDP(pkt.Bytes())
	}
	t.SetChecksum(0)
	sum := checksum.Combine(s.pseudoSum1, uint16(pkt.Data()))
	sum = checksum.Checksum(pkt.Bytes(), sum)
	t.SetChecksum(^sum)

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
