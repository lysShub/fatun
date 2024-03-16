package capture

import (
	"context"
	"log/slog"
	"os"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/client/filter"
	sess "github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Capture interface {
	Get(ctx context.Context) (Session, error)
	Close() error
}

type Session interface {
	Capture(ctx context.Context, pkt *relraw.Packet) (err error)
	Inject(pkt *relraw.Packet) error

	Session() sess.Session
	String() string
	Close() error
}

func NewCapture(hit filter.Hitter) (Capture, error) {
	return newCapture(hit, &Option{
		Prots:    []itun.Proto{itun.TCP},
		Logger:   slog.New(slog.NewJSONHandler(os.Stderr, nil)),
		Priority: 0,
		Mtu:      1536,
	}), nil
}

type Option struct {
	// NIC  int
	// IPv6 bool

	Prots []itun.Proto // default tcp

	Logger *slog.Logger

	Priority int16

	Mtu int
}
