package capture

import (
	"context"
	"log/slog"

	"github.com/lysShub/itun/app/client/filter"
	sess "github.com/lysShub/itun/session"
	"github.com/lysShub/sockit/packet"
)

// todo: Capture 需要解决IP DF问题
//
//	tcp 可以控制MSS opt解决，如果需要代理UDP，则无法完美解决，
//	sconn并未限制IP MF， 但是需要把Send/Recv的pkt的容量设置得
//	足够大即可。
//
// 所以现在只能优化MTU问题。
type Capture interface {
	Capture(ctx context.Context) (Session, error)
	Close() error
}

type Session interface {
	Capture(ctx context.Context, pkt *packet.Packet) (err error)
	Inject(pkt *packet.Packet) error

	Session() sess.Session
	String() string
	Close() error
}

func NewCapture(hit filter.Hitter, cfg *Config) (Capture, error) {
	return newCapture(hit, cfg)
}

type Config struct {
	// NIC  int
	// IPv6 bool

	Logger *slog.Logger

	Priority int16

	Mtu int
}
