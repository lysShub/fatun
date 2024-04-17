//go:build linux
// +build linux

package capture

import (
	"context"
	"time"

	"github.com/lysShub/itun/app/client/filter"
	"github.com/lysShub/sockit/packet"

	"github.com/pkg/errors"
)

type capture struct {
}

func newCapture(hit filter.Hitter, opt *Option) (Capture, error) {
	// return nil, errors.New("implement")
	panic(errors.New("todo implement"))
}

func (c *capture) Capture(ctx context.Context, pkt *packet.Packet) (err error) {
	time.Sleep(time.Hour)
	return nil
}
func (c *capture) Inject(pkt *packet.Packet) error {
	return nil
}
func (c *capture) Close() error {
	return nil
}
