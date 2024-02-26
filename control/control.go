package control

import (
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/link/channel"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Controller struct {
	link  *channel.Endpoint // todo: link store in Ustack
	stack *ustack.Ustack

	handshakeTimeout time.Duration

	closed atomic.Bool
}

func NewController(laddr, raddr netip.AddrPort, mtu int) (*Controller, error) {
	var c = &Controller{
		link:             channel.New(4, uint32(mtu), ""),
		handshakeTimeout: time.Hour, // todo: from cfg
	}

	var err error
	c.stack, err = ustack.NewUstack(c.link, laddr, raddr)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Controller) OutboundService(ctx cctx.CancelCtx, conn *sconn.Conn) {
	mtu := conn.Raw().MTU()
	head := 64

	p := relraw.NewPacket(head, mtu)
	for {
		p.Sets(head, mtu)
		err := c.stack.Outbound(ctx, p)
		if err != nil {
			ctx.Cancel(err)
			return
		}

		// strip ip header
		switch header.IPVersion(p.Data()) {
		case 4:
			n := int(header.IPv4(p.Data()).HeaderLength())
			p.SetHead(p.Head() + n)
		case 6:
			p.SetHead(p.Head() + header.IPv6MinimumSize)
		}

		seg := segment.ToSegment(p)
		seg.SetID(segment.CtrSegID)

		err = conn.SendSeg(ctx, seg)
		if err != nil {
			ctx.Cancel(err)
			return
		}
	}
}

func (c *Controller) Inbound(seg *segment.Segment) {
	// strip segment header
	seg.SetHead(seg.Head() + segment.HdrSize)

	c.stack.Inbound(seg.Packet())
}

func (c *Controller) Destroy() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}

	c.stack.Destroy()
	c.link.Close()
}
