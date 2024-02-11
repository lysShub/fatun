package sconn

import (
	"context"
	"errors"
	"fmt"
	"itun/segment"
	"net"

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
type MgrConn struct {
	conn net.Conn

	link  *channel.Endpoint
	stack *stack.Stack

	proto tcpip.NetworkProtocolNumber
	ip    *relraw.IPStack2
}

func AcceptMgrConn(ctx context.Context, raw *Conn) (*MgrConn, error) {
	mc, err := newMgrConn(ctx, raw)
	if err != nil {
		return nil, err
	}

	l, err := gonet.ListenTCP(mc.stack, raw.LocalAddr(), mc.proto)
	if err != nil {
		return nil, err
	}
	mc.conn, err = l.Accept()
	if err != nil {
		return nil, err
	}

	return mc, nil
}

func ConnectMgrConn(ctx context.Context, raw *Conn) (*MgrConn, error) {
	mc, err := newMgrConn(ctx, raw)
	if err != nil {
		return nil, err
	}

	mc.conn, err = gonet.DialTCPWithBind(ctx, mc.stack, raw.LocalAddr(), raw.RemoteAddr(), mc.proto)
	if err != nil {
		return nil, err
	}

	return mc, nil
}

func newMgrConn(ctx context.Context, raw *Conn) (*MgrConn, error) {
	var mc = &MgrConn{}

	var npf stack.NetworkProtocolFactory
	switch p := raw.NetworkProtocolNumber(); p {
	case header.IPv4ProtocolNumber:
		mc.proto = p
		npf = ipv4.NewProtocol
	case header.IPv6ProtocolNumber:
		mc.proto = p
		npf = ipv6.NewProtocol
	default:
		return nil, fmt.Errorf("invalid network protocol number %d", p)
	}

	mc.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{npf},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		// HandleLocal:        true,
	})
	mc.link = channel.New(4, uint32(raw.MTU()), "")

	const nicid tcpip.NICID = 5678
	if err := mc.stack.CreateNIC(nicid, mc.link); err != nil {
		return nil, errors.New(err.String())
	}
	mc.stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: raw.LocalAddr().Addr.WithPrefix(),
	}, stack.AddressProperties{})
	mc.stack.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})

	mc.ip = relraw.NewIPStack2(
		raw.LocalAddrAddrPort().Addr(),
		raw.RemoteAddrAddrPort().Addr(),
		tcp.ProtocolNumber,
	)

	go mc.downlink(ctx, raw)
	return mc, nil
}

func (mc *MgrConn) Inject(seg segment.Segment) {
	if seg.ID() != segment.MgrSegID {
		panic(fmt.Sprintf("not MgrSeg with id %d", seg.ID()))
	}

	var ip = make([]byte, len(seg.Payload())+mc.ip.AttachSize())
	copy(ip[mc.ip.AttachSize():], seg.Payload())

	mc.ip.AttachUp(ip)

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(ip)})

	mc.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
}

func (mc *MgrConn) downlink(ctx context.Context, raw *Conn) {
	for {
		pkb := mc.link.ReadContext(ctx)
		if pkb.IsNil() {
			return
		}

		iphdr := header.IPv4(pkb.ToView().AsSlice())
		n := iphdr.HeaderLength()
		segment.Segment(iphdr[n-1:]).SetID(segment.MgrSegID)

		err := raw.Write(segment.Segment(iphdr), int(n)-1)
		if err != nil {
			panic(err)
		}
		pkb.DecRef()
	}
}

func (mc *MgrConn) Next() (segment.MgrSeg, error) {
	return segment.ReadMgrMsg(mc.conn)
}

func (mc *MgrConn) Replay(seg segment.MgrSeg) error {
	_, err := mc.conn.Write(seg)
	return err
}
