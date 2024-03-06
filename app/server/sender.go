package server

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
)

type Sender interface {
	Send(pkt *relraw.Packet) error
	Recv(ctx context.Context, pkt *relraw.Packet) error

	Close() error
}

func NewSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (Sender, error) {
	return newSender(loc, proto, dst)
}
