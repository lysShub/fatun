package sender

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/rsocket"
)

type Sender interface {
	Send(pkt *rsocket.Packet) error
	Recv(ctx context.Context, pkt *rsocket.Packet) error

	Close() error
}

func NewSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (Sender, error) {
	return newSender(loc, proto, dst)
}
