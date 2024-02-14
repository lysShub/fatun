package itun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"runtime"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

type userStack struct {
	stack *stack.Stack
	link  *channel.Endpoint

	unreliable bool
}

const nicid tcpip.NICID = 123

func newUserStack(addr netip.Addr) (userStack, error) {
	var s = userStack{}

	s.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		// HandleLocal:        true,
	})
	link := channel.New(4, uint32(1500), "")
	if err := s.stack.CreateNIC(nicid, link); err != nil {
		return s, errors.New(err.String())
	}
	s.stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: tcpip.AddrFromSlice(addr.AsSlice()).WithPrefix(),
	}, stack.AddressProperties{})
	s.stack.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})
	for !link.IsAttached() {
		runtime.Gosched()
	}

	return s, nil
}

// LinkInject downlink, inject IP packet to stack
func (t *userStack) LinkInject(ip header.IPv4) (int, error) {
	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(ip)})
	defer pkb.DecRef()

	t.link.InjectInbound(ipv4.ProtocolNumber, pkb)
	return len(ip), nil
}

// LinkRead uplink, read IP packet from stack
func (t *userStack) LinkRead(ip header.IPv4) (int, error) {
	pkb := t.link.ReadContext(context.Background())
	defer pkb.DecRef()

	s := pkb.ToView().AsSlice()
	n := copy(ip, s)
	if n < len(s) {
		return 0, io.ErrShortBuffer
	}

	if t.unreliable {
		// tcphdr := header.TCP(ip[:n].Payload())

	}

	return n, nil
}

type TCPConn struct {
	userStack

	net.Conn
}

func DialUserTCP(laddr, raddr netip.AddrPort) (*TCPConn, error) {
	if laddr.Addr().Is6() || raddr.Addr().Is6() {
		return nil, fmt.Errorf("not support ipv6")
	}

	var c = &TCPConn{}

	var err error
	c.userStack, err = newUserStack(laddr.Addr())
	if err != nil {
		return nil, err
	}

	c.Conn, err = gonet.DialTCPWithBind(
		context.Background(),
		c.stack,
		tcpip.FullAddress{
			NIC:  nicid,
			Addr: tcpip.AddrFromSlice(laddr.Addr().AsSlice()),
			Port: uint16(laddr.Port()),
		},
		tcpip.FullAddress{
			NIC:  nicid,
			Addr: tcpip.AddrFromSlice(raddr.Addr().AsSlice()),
			Port: uint16(raddr.Port()),
		},
		ipv4.ProtocolNumber,
	)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func AcceptUserTCP(laddr, raddr netip.AddrPort) (*TCPConn, error) {
	if laddr.Addr().Is6() || raddr.Addr().Is6() {
		return nil, fmt.Errorf("not support ipv6")
	}

	var c = &TCPConn{}

	var err error
	c.userStack, err = newUserStack(laddr.Addr())
	if err != nil {
		return nil, err
	}

	l, err := gonet.ListenTCP(
		c.stack,
		tcpip.FullAddress{
			NIC:  nicid,
			Addr: tcpip.AddrFromSlice(laddr.Addr().AsSlice()),
			Port: uint16(laddr.Port()),
		},
		ipv4.ProtocolNumber,
	)
	if err != nil {
		return nil, err
	}

	conn, err := l.Accept()
	if err != nil {
		return nil, err
	}
	if r, err := netip.ParseAddrPort(conn.RemoteAddr().String()); err != nil {
		return nil, err
	} else if r != raddr {
		return nil, fmt.Errorf("expect remote address %s, got %s", raddr, r)
	}

	c.Conn = conn
	return c, nil
}
