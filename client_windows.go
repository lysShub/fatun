//go:build windows
// +build windows

package fatun

import (
	"fmt"
	"net/netip"
	"sync/atomic"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/errorx"
	mapping "github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type capture struct {
	capture *divert.Handle
	inbound divert.Address

	process atomic.Pointer[string]
	mapping mapping.Mapping

	overhead int
}

func NewDefaultCapture(laddr netip.AddrPort, overhead int) (Capturer, error) {
	var c = &capture{overhead: overhead}
	var err error

	var filter = fmt.Sprintf("outbound and !loopback and ip and (tcp or udp) and tcp.SrcPort!=%d", laddr.Port())
	c.capture, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, c.close(err)
	}
	if ifi, err := ifaceByAddr(laddr.Addr()); err != nil {
		return nil, err
	} else {
		c.inbound.SetOutbound(false)
		c.inbound.Network().IfIdx = uint32(ifi.Index)
	}
	if c.mapping, err = mapping.New(); err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func (c *capture) close(cause error) error {
	if c.capture != nil {
		if err := c.capture.Close(); err != nil && cause == nil {
			cause = err
		}
	}
	if c.mapping != nil {
		if err := c.mapping.Close(); err != nil && cause == nil {
			cause = err
		}
	}
	return cause
}

func (c *capture) Enable(process string) { c.process.Store(&process) }

func (c *capture) Capture(ip *packet.Packet) error {
	var addr divert.Address

	head, data := ip.Head(), ip.Data()
	for {
		n, err := c.capture.Recv(ip.Sets(head, data).Bytes(), &addr)
		if err != nil {
			return c.close(err)
		} else if n == 0 {
			continue
		}
		ip.SetData(n)

		s, err := FromIP(ip.Bytes())
		if err != nil {
			return err
		}
		pass := s.Dst.Addr().IsMulticast()
		if !pass {
			name, err := c.mapping.Name(s.Src, uint8(s.Proto))
			if err != nil {
				if errorx.Temporary(err) {
					// todo: logger
					pass = true
				} else {
					return c.close(err)
				}
			} else {
				pass = name != *c.process.Load()
			}
		}
		if pass {
			if _, err = c.capture.Send(ip.Bytes(), &addr); err != nil {
				return c.close(err)
			}
			continue
		}

		if s.Proto == header.TCPProtocolNumber {
			UpdateTcpMssOption(header.IPv4(ip.Bytes()).Payload(), -c.overhead)
		}
		return nil
	}
}

func (c *capture) Inject(ip *packet.Packet) error {
	if header.IPv4(ip.Bytes()).TransportProtocol() == header.TCPProtocolNumber {
		UpdateTcpMssOption(header.IPv4(ip.Bytes()).Payload(), -c.overhead)
	}

	_, err := c.capture.Send(ip.Bytes(), &c.inbound)
	return err
}
func (c *capture) Close() error { return c.close(nil) }
