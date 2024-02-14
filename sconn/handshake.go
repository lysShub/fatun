package sconn

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/fake/link"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

type ustack struct {
	raw   itun.RawConn
	stack *stack.Stack
	link  *link.Endpoint
}

func newUserStack(ctx context.Context, cancel context.CancelCauseFunc, raw *itun.RawConn) (*ustack, error) {

	st := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		// HandleLocal:        true,
	})
	l := link.New(4, uint32(raw.MTU()))

	const nicid tcpip.NICID = 1234
	if err := st.CreateNIC(nicid, l); err != nil {
		return nil, errors.New(err.String())
	}
	st.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: raw.LocalAddr().Addr.WithPrefix(),
	}, stack.AddressProperties{})
	st.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})

	var u = &ustack{
		stack: st,
		link:  l,
	}

	go u.uplink(ctx, cancel, raw)
	go u.downlink(ctx, cancel, raw)
	return u, nil
}

func (u *ustack) uplink(ctx context.Context, cancel context.CancelCauseFunc, raw *itun.RawConn) {
	var b = make([]byte, raw.MTU())

	// todo: RawConn support context
	for {
		n, err := raw.Read(b)
		if err != nil {
			select {
			case <-ctx.Done():
			default:
				cancel(fmt.Errorf("uplink %s", err.Error()))
			}
			return
		}
		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(b[:n])})
		u.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
	}
}

func (u *ustack) downlink(ctx context.Context, cancel context.CancelCauseFunc, raw *itun.RawConn) {
	for {
		pkb := u.link.ReadContext(ctx)
		if pkb.IsNil() {
			return // ctx cancel
		}

		_, err := raw.Write(pkb.ToView().AsSlice())
		if err != nil {
			cancel(fmt.Errorf("downlink %s", err.Error()))
			return
		}
	}
}

func (s *ustack) SeqAck() (seg, ack uint32) {
	return s.link.SeqAck()
}

func (s *ustack) Accept(ctx context.Context) (net.Conn, error) {
	l, err := gonet.ListenTCP(s.stack, s.raw.LocalAddr(), s.raw.NetworkProtocolNumber())
	if err != nil {
		return nil, err
	}

	conn, err := l.Accept()
	if err != nil {
		return nil, err
	}

	// todo: validate remote addr
	return conn, nil
}

func (s *ustack) Connect(ctx context.Context) (net.Conn, error) {
	return gonet.DialTCPWithBind(
		ctx, s.stack,
		s.raw.LocalAddr(), s.raw.RemoteAddr(),
		s.raw.NetworkProtocolNumber(),
	)
}
