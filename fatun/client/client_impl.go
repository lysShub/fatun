//go:build windows
// +build windows

package client

import (
	"log/slog"
	"net/netip"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type captureImpl Client
type captureImplPtr = *captureImpl

func (c *captureImpl) raw() *Client          { return ((*Client)(c)) }
func (c *captureImpl) Logger() *slog.Logger  { return c.config.Logger }
func (c *captureImpl) MaxRecvBuffSize() int  { return c.config.MaxRecvBuffSize }
func (c *captureImpl) DivertPriority() int16 { return c.divertPriority + 2 } // capture should read firstly
func (c *captureImpl) Hit(ip *packet.Packet) bool {
	hit, err := c.hiter.Hit(ip)
	if err != nil {
		if errorx.Temporary(err) {
			c.config.Logger.Warn(err.Error())
			// todo: should proxy on strict mode
		} else {
			c.raw().close(err)
		}
	} else if hit {

		// calc checksum:
		// regarded source IP/Port as zore value to calculate the transport checksum,
		// and then directly set the checksum (don't ^ operation)
		hdr := header.IPv4(ip.Bytes())
		switch hdr.TransportProtocol() {
		case header.TCPProtocolNumber:
			tcp := header.TCP(hdr.Payload())

			// reduce tcp mss, avoid ip split fragment
			if tcp.Flags().Contains(header.TCPFlagSyn) {
				fatun.UpdateMSS(tcp, -sconn.Overhead)
			}

			tcp.SetChecksum(0)
			srcPort := tcp.SourcePort()
			tcp.SetSourcePort(0)
			sum := header.PseudoHeaderChecksum(
				hdr.TransportProtocol(),
				defaultip4,
				hdr.DestinationAddress(),
				uint16(len(tcp)),
			)
			tcp.SetChecksum(checksum.Checksum(tcp, sum))
			tcp.SetSourcePort(srcPort)
		case header.UDPProtocolNumber:
			udp := header.UDP(hdr.Payload())
			udp.SetChecksum(0)
			srcPort := udp.SourcePort()
			udp.SetSourcePort(0)
			sum := header.PseudoHeaderChecksum(
				hdr.TransportProtocol(),
				defaultip4,
				hdr.DestinationAddress(),
				uint16(len(udp)),
			)
			udp.SetChecksum(checksum.Checksum(udp, sum))
			udp.SetSourcePort(srcPort)
		default:
			panic("")
		}
		ip.SetHead(ip.Head() + int(hdr.HeaderLength()))

		id := session.ID{
			Remote: netip.AddrFrom4(hdr.DestinationAddress().As4()),
			Proto:  hdr.TransportProtocol(),
		}
		if p := c.pcap.Load(); p != nil {
			if err := p.Outbound(id.Remote, id.Proto, ip.Bytes()); err != nil {
				panic(err)
			}
		}

		if err = c.raw().uplink(c.srvCtx, ip, id); err != nil {
			if errorx.Temporary(err) {
				c.config.Logger.Warn(err.Error())
			} else {
				c.raw().close(err)
			}
		}
	}
	return hit
}

var defaultip4 = tcpip.AddrFrom4([4]byte{})
