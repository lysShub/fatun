package control

import (
	"errors"
	"fmt"
	"net"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/segment"

	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

// manager connection
type CtrConn struct {
	net.Conn

	link  *channel.Endpoint
	stack *stack.Stack

	proto   tcpip.NetworkProtocolNumber
	ipstack *relraw.IPStack
}

func AcceptCtrConn(ctx cctx.CancelCtx, conn *sconn.Conn) *CtrConn {
	mc := newCtrConn(ctx, conn)
	if ctx.Err() != nil {
		return nil
	}

	l, err := gonet.ListenTCP(mc.stack, conn.Raw().LocalAddr(), mc.proto)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}
	mc.Conn, err = l.Accept()
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	return mc
}

func ConnectCtrConn(ctx cctx.CancelCtx, conn *sconn.Conn) *CtrConn {
	mc := newCtrConn(ctx, conn)
	if ctx.Err() != nil {
		return nil
	}

	var err error
	mc.Conn, err = gonet.DialTCPWithBind(
		ctx, mc.stack,
		conn.Raw().LocalAddr(),
		conn.Raw().RemoteAddr(),
		mc.proto,
	)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	return mc
}

func newCtrConn(ctx cctx.CancelCtx, conn *sconn.Conn) *CtrConn {
	var mc = &CtrConn{}

	var npf stack.NetworkProtocolFactory
	switch p := conn.Raw().NetworkProtocolNumber(); p {
	case header.IPv4ProtocolNumber:
		mc.proto = p
		npf = ipv4.NewProtocol
	case header.IPv6ProtocolNumber:
		mc.proto = p
		npf = ipv6.NewProtocol
	default:
		ctx.Cancel(fmt.Errorf("invalid network protocol number %d", p))
		return nil
	}

	mc.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{npf},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		// HandleLocal:        true,
	})
	mc.link = channel.New(4, uint32(conn.Raw().MTU()), "")

	const nicid tcpip.NICID = 5678
	if err := mc.stack.CreateNIC(nicid, mc.link); err != nil {
		ctx.Cancel(errors.New(err.String()))
		return nil
	}
	mc.stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: conn.Raw().LocalAddr().Addr.WithPrefix(),
	}, stack.AddressProperties{})
	mc.stack.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})

	var err error
	mc.ipstack, err = relraw.NewIPStack(
		conn.Raw().LocalAddrPort().Addr(),
		conn.Raw().RemoteAddrPort().Addr(),
		tcp.ProtocolNumber,
	)
	if err != nil {
		ctx.Cancel(err)
	}

	go mc.downlink(ctx, conn)
	return mc
}

func (mc *CtrConn) Inject(seg *segment.Segment) {
	if seg.ID() != segment.CtrSegID {
		panic(fmt.Sprintf("not MgrSeg with id %d", seg.ID()))
	}

	// remove ip header and segment header
	p := seg.Packet()
	p.SetHead(p.Head() + segment.HdrSize)

	mc.ipstack.AttachInbound(p)

	// todo: validate ip packet

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(p.Data()),
	})
	mc.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
}

func (mc *CtrConn) downlink(ctx cctx.CancelCtx, conn *sconn.Conn) {
	for {
		pkb := mc.link.ReadContext(ctx)
		if pkb.IsNil() {
			return
		}
		iphdr := header.IPv4(pkb.ToView().AsSlice())

		p := relraw.ToPacket(int(iphdr.HeaderLength()), iphdr) // todo: optimize

		seg := segment.ToSegment(p)
		seg.SetID(segment.CtrSegID)

		err := conn.SendSeg(ctx, seg)
		if err != nil {
			ctx.Cancel(err)
		}
		pkb.DecRef()
	}
}
