package sconn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/fake/link"
	"github.com/lysShub/relraw"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

type ustack struct {
	raw   *itun.RawConn
	stack *stack.Stack
	link  *link.Endpoint
}

func newUserStack(ctx cctx.CancelCtx, raw *itun.RawConn) *ustack {

	st := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		HandleLocal:        false,
	})
	l := link.New(4, uint32(raw.MTU()))

	const nicid tcpip.NICID = 1234
	if err := st.CreateNIC(nicid, l); err != nil {
		ctx.Cancel(errors.New(err.String()))
		return nil
	}
	st.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: raw.LocalAddr().Addr.WithPrefix(),
	}, stack.AddressProperties{})
	st.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})

	var u = &ustack{
		raw:   raw,
		stack: st,
		link:  l,
	}

	go u.uplink(ctx, raw)
	go u.downlink(ctx, raw)
	return u
}

func (u *ustack) uplink(ctx cctx.CancelCtx, raw *itun.RawConn) {
	mtu := raw.MTU()
	var p = relraw.ToPacket(0, make([]byte, mtu))

	for {
		p.Sets(0, mtu)
		err := raw.ReadCtx(ctx, p)
		if err != nil {
			select {
			case <-ctx.Done():
			default:
				ctx.Cancel(fmt.Errorf("uplink %s", err.Error()))
			}
			return
		}

		{
			// todo: attach
			// todo: 看能不直接
			p.SetHead(0)
			iphdr := header.IPv4(p.Data())
			tcphdr := header.TCP(iphdr.Payload())
			fmt.Printf(
				"%s:%d-->%s:%d	\n",
				iphdr.SourceAddress(), tcphdr.SourcePort(),
				iphdr.DestinationAddress(), tcphdr.DestinationPort(),
			)

			fmt.Printf(
				"%s-->%s	\n",
				u.raw.RemoteAddrPort().String(),
				u.raw.LocalAddrPort().String(),
			)

		}

		// recover tcp to ip
		p.SetHead(0)

		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(p.Data()),
		})
		u.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
	}
}

func (u *ustack) downlink(ctx cctx.CancelCtx, raw *itun.RawConn) {
	for {
		pkb := u.link.ReadContext(ctx)
		if pkb.IsNil() {
			return // ctx cancel
		}

		_, err := raw.Write(pkb.ToView().AsSlice())
		if err != nil {
			ctx.Cancel(fmt.Errorf("downlink %s", err.Error()))
			return
		}
	}
}

func (s *ustack) SeqAck() (seg, ack uint32) {
	return s.link.SeqAck()
}

func (s *ustack) Accept(ctx cctx.CancelCtx) net.Conn {
	l, err := gonet.ListenTCP(s.stack, s.raw.LocalAddr(), s.raw.NetworkProtocolNumber())
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	acceptCtx := cctx.WithTimeout(ctx, time.Second*5) // todo: from config

	var conn net.Conn
	go func() {
		var err error
		conn, err = l.Accept()
		if err != nil {
			acceptCtx.Cancel(err)
		}
		acceptCtx.Cancel(nil)
	}()

	<-acceptCtx.Done()
	if err = acceptCtx.Err(); !errors.Is(err, context.Canceled) {
		ctx.Cancel(errors.Join(err, l.Close()))
		return nil
	}

	return conn // todo: validate remote addr
}

func (s *ustack) Connect(ctx cctx.CancelCtx) net.Conn {
	connectCtx := cctx.WithTimeout(ctx, time.Second*5) // todo: from config

	var conn net.Conn
	go func() { // gonet context is fool
		var err error
		conn, err = gonet.DialTCPWithBind(
			connectCtx, s.stack,
			s.raw.LocalAddr(), s.raw.RemoteAddr(),
			s.raw.NetworkProtocolNumber(),
		)
		if err != nil {
			ctx.Cancel(err)
		}
		connectCtx.Cancel(nil)
	}()

	<-connectCtx.Done()
	if err := connectCtx.Err(); !errors.Is(err, context.Canceled) {
		ctx.Cancel(err)
	}

	return conn
}
