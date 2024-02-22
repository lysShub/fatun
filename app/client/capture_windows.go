package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/netip"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func NewCapture(s itun.Session) (Capture, error) {
	var c = &capture{}
	switch s.Proto {
	case itun.TCP:
		c.minSize += header.TCPMinimumSize
	case itun.UDP:
		c.minSize += header.UDPMinimumSize
	case itun.ICMP:
		c.minSize += header.ICMPv4MinimumSize
	case itun.ICMPV6:
		c.minSize += header.ICMPv6MinimumSize
	default:
		panic("")
	}
	if s.SrcAddr.Addr().Is4() {
		c.minSize += header.IPv4MinimumSize
	} else {
		c.minSize += header.IPv6MinimumSize
	}

	var err error
	c.ip, err = relraw.NewIPStack(
		s.SrcAddr.Addr(), s.DstAddr.Addr(),
		tcpip.TransportProtocolNumber(s.Proto),
		// todo: option
	)
	if err != nil {
		return nil, err
	}

	filter := fmt.Sprintf(
		"%s and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d",
		s.Proto, s.SrcAddr.Addr(), s.SrcAddr.Port(), s.DstAddr.Addr(), s.DstAddr.Port(),
	)

	c.hdl, err = divert.Open(filter, divert.NETWORK, 0, divert.READ_ONLY|divert.WRITE_ONLY)
	if err != nil {
		return nil, err
	}

	if nic, err := getAddrNIC(s.SrcAddr.Addr()); err != nil {
		return nil, err
	} else {
		var inbound divert.Address
		inbound.SetOutbound(false)
		inbound.Network().IfIdx = uint32(nic)
		c.inboundAddr = inbound
	}

	return c, nil
}

type capture struct {
	hdl *divert.Divert

	ip *relraw.IPStack

	minSize int

	inboundAddr divert.Address
}

type ErrCaptureInvalidPacket string

func (e ErrCaptureInvalidPacket) Error() string {
	return fmt.Sprintf("capture invalid packet：%s", string(e))
}

func (c *capture) RecvCtx(ctx context.Context, p *relraw.Packet) (err error) {
	ip := p.Data()
	ip = ip[:cap(ip)]

	n, err := c.hdl.RecvCtx(ctx, ip, nil)
	if err != nil {
		return err
	} else if n == 0 {
		return c.RecvCtx(ctx, p)
	} else if n < c.minSize {
		return ErrCaptureInvalidPacket(hex.EncodeToString(ip[:n]))
	}

	p.SetLen(n)

	iphdrLen := 0
	switch header.IPVersion(ip) {
	case 4:
		iphdrLen = int(header.IPv4(ip).HeaderLength())
	case 6:
		iphdrLen = header.IPv6MinimumSize
	}

	p.SetHead(p.Head() + iphdrLen)
	return nil
}

func (c *capture) Inject(p *relraw.Packet) error {

	c.ip.AttachInbound(p)

	_, err := c.hdl.Send(p.Data(), &c.inboundAddr)
	return err
}

func (c *capture) Close() error { return c.hdl.Close() }

func getAddrNIC(addr netip.Addr) (int, error) {
	panic("todo")
	return 0, nil
}
