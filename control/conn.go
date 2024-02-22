package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

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

type CtrInject interface {
	Inject(tcp *relraw.Packet)
}

// todo： merge sconn user stack
type Ustack struct {
	id    string
	link  *channel.Endpoint
	stack *stack.Stack

	proto   tcpip.NetworkProtocolNumber
	ipstack *relraw.IPStack
}

var _ CtrInject = (*Ustack)(nil)

func newUserStack(ctx cctx.CancelCtx, id string, conn *sconn.Conn) *Ustack {
	var mc = &Ustack{id: id}

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

	go mc.outbound(ctx, conn)
	return mc
}

func (mc *Ustack) Inject(tcp *relraw.Packet) {
	// if seg.ID() != segment.CtrSegID {
	// 	panic(fmt.Sprintf("not MgrSeg with id %d", seg.ID()))
	// }

	// // remove ip header and segment header
	// p := seg.Packet()
	// p.SetHead(p.Head() + segment.HdrSize)

	mc.ipstack.AttachInbound(tcp)

	// todo: validate ip packet

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(tcp.Data()),
	})
	mc.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
}

func (mc *Ustack) outbound(ctx cctx.CancelCtx, conn *sconn.Conn) {
	for {
		pkb := mc.link.ReadContext(ctx)
		if pkb.IsNil() {
			return
		}

		iphdr := header.IPv4(pkb.ToView().AsSlice())

		p := relraw.ToPacket(int(iphdr.HeaderLength()), iphdr) // todo: optimize

		{
			tcphdr := header.TCP(p.Data())
			if mc.id == server {
				fmt.Printf(
					"%s send %d-->%d	%s\n",
					mc.id,
					tcphdr.SourcePort(), tcphdr.DestinationPort(),
					tcphdr.Flags(),
				)

				// opts := tcphdr.Options()
				// fmt.Println(opts)

				if tcphdr.Flags().Contains(header.TCPFlagSyn | header.TCPFlagAck) {
					print()
				}
			} else {
				fmt.Printf(
					"%s send %d-->%d	%s\n",
					mc.id,
					tcphdr.SourcePort(), tcphdr.DestinationPort(),
					tcphdr.Flags(),
				)
			}
		}

		seg := segment.ToSegment(p)
		seg.SetID(segment.CtrSegID)

		err := conn.SendSeg(ctx, seg)
		if err != nil {
			ctx.Cancel(err)
		}
		pkb.DecRef()
	}
}

func connect(ctx cctx.CancelCtx, tcpHandshakeTimeout time.Duration, us *Ustack, laddr, raddr tcpip.FullAddress) (tcp net.Conn) {
	connectCtx := cctx.WithTimeout(ctx, tcpHandshakeTimeout)
	defer connectCtx.Cancel(nil)

	tcp, err := gonet.DialTCPWithBind(
		connectCtx, us.stack,
		laddr, raddr,
		us.proto,
	)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	return tcp
}

func accept(
	ctx cctx.CancelCtx, tcpHandshakeTimeout time.Duration,
	us *Ustack,
	laddr, raddr tcpip.FullAddress,
) (tcp net.Conn) {
	l, err := gonet.ListenTCP(us.stack, laddr, us.proto)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	acceptCtx := cctx.WithTimeout(ctx, tcpHandshakeTimeout)

	go func() {
		var err error
		tcp, err = l.Accept()
		acceptCtx.Cancel(err)
	}()

	<-acceptCtx.Done()
	if err = acceptCtx.Err(); !errors.Is(err, context.Canceled) {
		ctx.Cancel(errors.Join(err, l.Close()))
		return nil
	}

	return tcp // todo: validate remote addr
}
