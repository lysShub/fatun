//go:build windows
// +build windows

package capture

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

/*
	filter priority: 2 RS

	capture priority: 1 RW

*/

const (
	capturePriority = 1
)

type capture struct {
	hdl *divert.Divert

	ipstack *relraw.IPStack

	minSize     int
	inboundAddr divert.Address
}

func newCapture(s session.Session) (Capture, error) {
	var c = &capture{}

	if addr, nic, err := divert.Gateway(s.Src.Addr()); err != nil {
		return nil, err
	} else {
		s.Src = netip.AddrPortFrom(addr, s.Src.Port())

		var inbound divert.Address
		inbound.SetOutbound(false)
		inbound.Network().IfIdx = uint32(nic)
		c.inboundAddr = inbound
	}
	c.minSize = s.MinPacketSize()

	var err error
	c.ipstack, err = relraw.NewIPStack(
		s.Src.Addr(), s.Dst.Addr(),
		tcpip.TransportProtocolNumber(s.Proto),
		// todo: option
	)
	if err != nil {
		return nil, err
	}

	filter := fmt.Sprintf(
		"%s and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d",
		strings.ToLower(s.Proto.String()), s.Src.Addr(), s.Src.Port(), s.Dst.Addr(), s.Dst.Port(),
	)

	// todo: support forway
	c.hdl, err = divert.Open(filter, divert.NETWORK, capturePriority, 0)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *capture) Capture(ctx context.Context, pkg *relraw.Packet) (err error) {
	ip := pkg.Data()
	ip = ip[:cap(ip)]

	n, err := c.hdl.RecvCtx(ctx, ip, nil)
	if err != nil {
		return err
	} else if n == 0 {
		return c.Capture(ctx, pkg)
	} else if n < c.minSize {
		return ErrCaptureInvalidPacket(hex.EncodeToString(ip[:n]))
	}

	pkg.SetLen(n)

	iphdrLen := 0
	switch header.IPVersion(ip) {
	case 4:
		iphdrLen = int(header.IPv4(ip).HeaderLength())
	case 6:
		iphdrLen = header.IPv6MinimumSize
	}

	pkg.SetHead(pkg.Head() + iphdrLen)

	return nil
}

func (c *capture) Inject(pkt *relraw.Packet) error {

	c.ipstack.AttachInbound(pkt)

	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Data())
	}

	_, err := c.hdl.Send(pkt.Data(), &c.inboundAddr)
	return err
}

func (c *capture) Close() error { return c.hdl.Close() }

type ErrCaptureInvalidPacket string

func (e ErrCaptureInvalidPacket) Error() string {
	return fmt.Sprintf("capture invalid packet：%s", string(e))
}
