//go:build linux
// +build linux

package client

import (
	"context"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
)

type capture struct {
}

func newCapture(s itun.Session) (Capture, error) {
	// return nil, errors.New("implement")
	return &capture{}, nil
}

func (c *capture) RecvCtx(ctx context.Context, p *relraw.Packet) (err error) {
	time.Sleep(time.Hour)
	return nil
}
func (c *capture) Inject(p *relraw.Packet) error {
	return nil
}
func (c *capture) Close() error {
	return nil
}
