package control

import (
	"context"
	"net/netip"

	"github.com/lysShub/itun/cctx"
)

type Client interface {
	Close() error

	IPv6(ctx context.Context) (bool, error)
	EndConfig(ctx context.Context) error
	AddTCP(ctx context.Context, addr netip.AddrPort) (uint16, error)
	DelTCP(ctx context.Context, id uint16) error
	AddUDP(ctx context.Context, addr netip.AddrPort) (uint16, error)
	DelUDP(ctx context.Context, id uint16) error
	PackLoss(ctx context.Context) (float32, error)
	Ping(ctx context.Context) error
}

func Dial(ctx cctx.CancelCtx, ctr *Controller) Client {
	tcp := ctr.stack.Connect(ctx, ctr.handshakeTimeout)
	if ctx.Err() != nil {
		return nil
	}

	return newGrpcClient(ctx, ctr, tcp, ctr.handshakeTimeout*3) // todo: from cfg
}
