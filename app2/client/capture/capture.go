package capture

import (
	"context"

	"github.com/lysShub/itun/app2/client/filter"
	"github.com/lysShub/itun/cctx"
	sess "github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Capture interface {
	Get(ctx context.Context) (Session, error)
	Del(s sess.Session)
}

type Session interface {
	Capture(ctx context.Context, pkt *relraw.Packet) (err error)

	Inject(pkt *relraw.Packet) error

	Session() sess.Session
	String() string
}

func NewCapture(ctx cctx.CancelCtx, hit filter.Hitter) (Capture, error) {
	return newCapture(ctx, hit, nil), nil
}
