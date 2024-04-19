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

func NewSender(loc netip.AddrPort, proto tcpip.TransportProtocolNumber, dst netip.AddrPort) (Sender, error) {
	return newSender(loc, proto, dst)
}
