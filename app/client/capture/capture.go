package capture

import (
	"context"

	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Capture interface {
	Capture(ctx context.Context, pkt *relraw.Packet) (err error)

	Inject(pkt *relraw.Packet) error

	Close() error
}

func NewCapture(session session.Session) (Capture, error) {
	return newCapture(session)
}
