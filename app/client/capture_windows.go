//go:build windows
// +build windows

package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"

	pkge "github.com/pkg/errors"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

/*
	filter priority: 2 RS

	capture priority: 1 RW

*/

const (
	capturePriority = 1
	filterPriority  = 2
)

func newCapture(s itun.Session) (Capture, error) {
	var c = &capture{}

	if addr, nic, err := getAddrNIC(s.SrcAddr.Addr()); err != nil {
		return nil, err
	} else {
		s.SrcAddr = netip.AddrPortFrom(addr, s.SrcAddr.Port())

		var inbound divert.Address
		inbound.SetOutbound(false)
		inbound.Network().IfIdx = uint32(nic)
		c.inboundAddr = inbound
	}
	c.minSize = s.MinPacketSize()

	var err error
	c.ipstack, err = relraw.NewIPStack(
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

	c.hdl, err = divert.Open(filter, divert.NETWORK, capturePriority, divert.READ_ONLY|divert.WRITE_ONLY)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type capture struct {
	hdl *divert.Divert

	ipstack *relraw.IPStack

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

	// todo: remove tcp/udp checksum pseudo-sum party
	return nil
}

func (c *capture) Inject(p *relraw.Packet) error {

	c.ipstack.AttachInbound(p)

	_, err := c.hdl.Send(p.Data(), &c.inboundAddr)
	return err
}

func (c *capture) Close() error { return c.hdl.Close() }

func getAddrNIC(addr netip.Addr) (netip.Addr, int, error) {
	if addr.IsLoopback() {
		c, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53})
		if err != nil {
			return addr, 0, pkge.WithStack(err)
		}

		defAddr, err := netip.ParseAddrPort(c.LocalAddr().String())
		if err != nil {
			return addr, 0, pkge.WithStack(err)
		}
		addr = defAddr.Addr()
	}

	ifs, err := net.Interfaces()
	if err != nil {
		return addr, 0, err
	}

	for _, i := range ifs {
		as, err := i.Addrs()
		if err != nil {
			return addr, 0, pkge.WithStack(err)
		}
		for _, a := range as {
			var sub netip.Prefix
			switch a := a.(type) {
			case *net.IPAddr:
				if ip := a.IP.To4(); ip != nil {
					sub = netip.PrefixFrom(netip.AddrFrom4([4]byte(ip)), 32)
				} else {
					sub = netip.PrefixFrom(netip.AddrFrom16([16]byte(a.IP)), 128)
				}
			case *net.IPNet:
				sub, err = netip.ParsePrefix(a.String())
				if err != nil {
					continue
				}
			default:
				return addr, 0, pkge.Errorf("unknow address type %T", a)
			}

			if sub.Contains(addr) {
				return addr, i.Index, nil
			}
		}
	}

	return addr, 0, pkge.Errorf("invalid address %s", addr)
}
