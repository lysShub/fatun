package client

import "context"

type Capture interface {
	RecvCtx(ctx context.Context, ip []byte) (n int, err error)
	Inject(b []byte) error
}
