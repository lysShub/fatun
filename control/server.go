package control

import (
	"net/netip"

	"github.com/lysShub/itun/cctx"
)

type SrvHandler interface {
	IPv6() bool
	EndConfig()
	AddTCP(addr netip.AddrPort) (uint16, error)
	DelTCP(id uint16) error
	AddUDP(addr netip.AddrPort) (uint16, error)
	DelUDP(id uint16) error
	PackLoss() float32
	Ping()
}

func Serve(ctx cctx.CancelCtx, ctr *Controller, hander SrvHandler) {
	tcp := ctr.stack.Accept(ctx, ctr.handshakeTimeout)
	if ctx.Err() != nil {
		return
	}

	serveGrpc(ctx, tcp, hander)
}
