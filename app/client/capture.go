package client

import (
	"context"

	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
)

type Capture interface {
	// recv tcp/udp packet
	RecvCtx(ctx context.Context, p *relraw.Packet) (err error)

	// inject tcp/udp packet
	Inject(p *relraw.Packet) error

	Close() error
}

func NewCapture(session itun.Session) (Capture, error) {
	return newCapture(session)
}
