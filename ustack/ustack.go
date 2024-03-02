package ustack

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"time"

	pkge "github.com/pkg/errors"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

// user tcp stack
type Ustack struct {
	laddr, raddr tcpip.FullAddress
	proto        tcpip.NetworkProtocolNumber
	id           string

	stack *stack.Stack
	link  link.LinkEndpoint

	ipstack *relraw.IPStack
}

func NewUstack(
	link link.LinkEndpoint,
	laddr, raddr netip.AddrPort,
) (*Ustack, error) {

	var err error
	var u = &Ustack{
		laddr: tcpip.FullAddress{Addr: tcpip.AddrFrom4(laddr.Addr().As4()), Port: laddr.Port()},
		raddr: tcpip.FullAddress{Addr: tcpip.AddrFrom4(raddr.Addr().As4()), Port: raddr.Port()},
		link:  link,
	}
	var npf stack.NetworkProtocolFactory
	if laddr.Addr().Is4() {
		u.proto = header.IPv4ProtocolNumber
		npf = ipv4.NewProtocol
	} else {
		u.proto = header.IPv6ProtocolNumber
		npf = ipv6.NewProtocol
	}
	u.ipstack, err = relraw.NewIPStack(
		laddr.Addr(), raddr.Addr(),
		tcp.ProtocolNumber,
	//  todo: set opt
	)
	if err != nil {
		return nil, err
	}

	u.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{npf},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol}, // todo: set nProcessors 1
		HandleLocal:        false,
	})

	if err := u.stack.CreateNIC(nicid, link); err != nil {
		return nil, errors.New(err.String())
	}
	u.stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          u.proto,
		AddressWithPrefix: u.laddr.Addr.WithPrefix(),
	}, stack.AddressProperties{})
	u.stack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: nicid},
		{Destination: header.IPv6EmptySubnet, NIC: nicid},
	})

	return u, nil
}

const nicid tcpip.NICID = 1234

func (u *Ustack) SetID(id string) { u.id = id }
func (u *Ustack) ID() string      { return u.id }

func (u *Ustack) InboundRaw(ip *relraw.Packet) error {

	u.link.Inbound(ip)

	return nil
}

func (u *Ustack) Inbound(b *relraw.Packet) error {
	u.ipstack.AttachInbound(b)

	if debug.Debug() {
		test.ValidIP(test.T(), b.Data())
	}

	u.link.Inbound(b)

	return nil
}

func (u *Ustack) Outbound(ctx context.Context, ip *relraw.Packet) error {
	u.link.Outbound(ctx, ip)
	if ip.Len() == 0 {
		ip.SetLen(0)
		select {
		case <-ctx.Done():
			return ctx.Err() // ctx cancel/exceed
		default:
			return pkge.Wrap(os.ErrClosed, "user stack outbound")
		}
	}

	if debug.Debug() {
		test.ValidIP(test.T(), ip.Data())
	}

	return nil
}

func (u *Ustack) Accept(ctx cctx.CancelCtx, handshakeTimeout time.Duration) (conn net.Conn) {
	l, err := gonet.ListenTCP(u.stack, u.laddr, u.proto)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	acceptCtx := cctx.WithTimeout(ctx, handshakeTimeout)

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

	u.id = "server"
	return conn // todo: validate remote addr
}

func (u *Ustack) Connect(ctx cctx.CancelCtx, handshakeTimeout time.Duration) (conn net.Conn) {
	connectCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	var err error
	conn, err = gonet.DialTCPWithBind(
		connectCtx, u.stack,
		u.laddr, u.raddr,
		u.proto,
	)
	if err != nil {
		ctx.Cancel(err)
	}

	u.id = "client"
	return conn
}

func (s *Ustack) Closed() <-chan struct{} {
	return s.link.FinRstFlag()
}

// Destroy destroy user stack, avoid goroutine leak, ensure call after
// connect closed
func (s *Ustack) Destroy() {
	s.stack.Destroy()
}
