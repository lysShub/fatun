package sconn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
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

type TCP struct {
	*ustack
	net.Conn
}

func (t *TCP) Close() error {
	return errors.Join(t.Conn.Close(), t.ustack.Close())
}

func AcceptTCP(ctx cctx.CancelCtx, raw *itun.RawConn) *TCP {
	var tcp = &TCP{}

	var err error
	if tcp.ustack, err = newUserStack(
		ctx, raw.MTU(),
		raw.LocalAddr(), raw.RemoteAddr(),
		raw.NetworkProtocolNumber(),
	); err != nil {
		ctx.Cancel(err)
		return nil
	}

	go tcp.ustack.uplink(raw)
	go tcp.ustack.downlink(raw)

	tcp.Conn = tcp.ustack.accept(ctx)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	return tcp
}

func ConnectTCP(ctx cctx.CancelCtx, raw *itun.RawConn) *TCP {
	var tcp = &TCP{}

	var err error
	if tcp.ustack, err = newUserStack(
		ctx, raw.MTU(),
		raw.LocalAddr(), raw.RemoteAddr(),
		raw.NetworkProtocolNumber(),
	); err != nil {
		ctx.Cancel(err)
		return nil
	}

	go tcp.ustack.uplink(raw)
	go tcp.ustack.downlink(raw)

	tcp.Conn = tcp.ustack.connect(ctx)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil
	}

	return tcp
}

type ustack struct {
	ctx          cctx.CancelCtx
	laddr, raddr tcpip.FullAddress
	proto        tcpip.NetworkProtocolNumber

	stack *stack.Stack
	link  *link.Endpoint

	closed   atomic.Bool
	closedCh chan struct{}
}

func newUserStack(ctx cctx.CancelCtx, mtu int, laddr, raddr tcpip.FullAddress, proto tcpip.NetworkProtocolNumber) (*ustack, error) {

	st := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		HandleLocal:        false,
	})
	l := link.New(4, uint32(mtu))

	const nicid tcpip.NICID = 1234
	if err := st.CreateNIC(nicid, l); err != nil {
		return nil, errors.New(err.String())
	}
	st.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: laddr.Addr.WithPrefix(),
	}, stack.AddressProperties{})
	st.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})

	var u = &ustack{
		ctx:   cctx.WithContext(ctx),
		laddr: laddr,
		raddr: raddr,
		proto: proto,

		stack:    st,
		link:     l,
		closedCh: make(chan struct{}, 2),
	}

	return u, nil
}

func (u *ustack) uplink(raw *itun.RawConn) {
	mtu := raw.MTU()
	var p = relraw.ToPacket(0, make([]byte, mtu))

	for {
		p.Sets(0, mtu)
		err := raw.ReadCtx(u.ctx, p)
		if err != nil {
			u.ctx.Cancel(fmt.Errorf("user stack uplink %s", err.Error()))
			u.closedCh <- struct{}{}
			return
		}

		// recover tcp to ip
		p.SetHead(0)

		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(p.Data()),
		})
		u.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
	}
}

func (u *ustack) downlink(raw *itun.RawConn) {

	for {
		pkb := u.link.ReadContext(u.ctx)
		if pkb.IsNil() {
			u.closedCh <- struct{}{}
			return // ctx cancel
		}

		_, err := raw.Write(pkb.ToView().AsSlice())
		if err != nil {
			u.ctx.Cancel(fmt.Errorf("user stack downlink %s", err.Error()))
			u.closedCh <- struct{}{}
			return
		}
	}
}

func (s *ustack) SeqAck() (seg, ack uint32) {
	return s.link.SeqAck()
}

func (s *ustack) accept(ctx cctx.CancelCtx) (conn net.Conn) {
	l, err := gonet.ListenTCP(s.stack, s.laddr, s.proto)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	acceptCtx := cctx.WithTimeout(ctx, time.Second*5) // todo: from config

	go func() {
		var err error
		conn, err = l.Accept()
		acceptCtx.Cancel(err)
	}()

	<-acceptCtx.Done()
	if err = acceptCtx.Err(); !errors.Is(err, context.Canceled) {
		ctx.Cancel(errors.Join(err, l.Close()))
		return nil
	}

	return conn // todo: validate remote addr
}

func (s *ustack) connect(ctx cctx.CancelCtx) (conn net.Conn) {
	connectCtx, cancel := context.WithTimeout(ctx, time.Second*5) // todo: from config
	defer cancel()

	var err error
	conn, err = gonet.DialTCPWithBind(
		connectCtx, s.stack,
		s.laddr, s.raddr,
		s.proto,
	)
	if err != nil {
		ctx.Cancel(err)
	}

	return conn
}

func (s *ustack) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil // closed
	}

	s.stack.Close()
	s.link.Close()

	s.ctx.Cancel(nil)

	select {
	case <-s.closedCh:
	case <-time.After(time.Second * 3):
		return fmt.Errorf("user stack close timeout")
	}

	select {
	case <-s.closedCh:
	case <-time.After(time.Second * 3):
		return fmt.Errorf("user stack close timeout")
	}

	close(s.closedCh)
	return nil
}
