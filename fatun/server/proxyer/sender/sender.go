package sender

import (
	"context"
	"net/netip"

	"github.com/lysShub/sockit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
)

type Sender interface {
	Send(pkt *packet.Packet) error
	Recv(ctx context.Context, pkt *packet.Packet) error

	Close() error
}

func NewSender(local netip.AddrPort, proto tcpip.TransportProtocolNumber, remote netip.AddrPort) (Sender, error) {
	return newSender(local, proto, remote)
}
