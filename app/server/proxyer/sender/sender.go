package sender

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/sockit/packet"

)

type Sender interface {
	Send(pkt *packet.Packet) error
	Recv(ctx context.Context, pkt *packet.Packet) error

	Close() error
}

func NewSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (Sender, error) {
	return newSender(loc, proto, dst)
}
