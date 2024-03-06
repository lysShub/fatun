//go:build linux
// +build linux

package capture

import (
	"context"
	"time"

	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type capture struct {
}

func newCapture(s session.Session) (Capture, error) {
	// return nil, errors.New("implement")
	return &capture{}, nil
}

func (c *capture) Capture(ctx context.Context, pkt *relraw.Packet) (err error) {
	time.Sleep(time.Hour)
	return nil
}
func (c *capture) Inject(pkt *relraw.Packet) error {
	return nil
}
func (c *capture) Close() error {
	return nil
}
