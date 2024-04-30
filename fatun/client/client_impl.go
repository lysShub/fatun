//go:build windows
// +build windows

package client

import (
	"log/slog"
	"net/netip"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type captureImpl Client
type captureImplPtr = *captureImpl

func (c *captureImpl) raw() *Client          { return ((*Client)(c)) }
func (c *captureImpl) Logger() *slog.Logger  { return c.logger }
func (c *captureImpl) MTU() int              { return c.cfg.MTU }
func (c *captureImpl) DivertPriority() int16 { return c.divertPriority - 2 } // capture should read firstly
func (c *captureImpl) Hit(ip *packet.Packet) bool {
	hit, err := c.hiter.Hit(ip)
	if err != nil {
		if errorx.Temporary(err) {
			c.logger.Warn(err.Error(), errorx.TraceAttr(err))
		} else {
			c.raw().close(err)
		}
	} else if hit {
		// todo: set by config
		// calc checksum
		hdr := header.IPv4(ip.Bytes())

		hdr.SetChecksum(^hdr.CalculateChecksum())
		if debug.Debug() {
			require.Equal(test.T(), 4, header.IPVersion(ip.Bytes()))
			test.ValidIP(test.T(), ip.Bytes())
		}

		ip.SetHead(ip.Head() + int(hdr.HeaderLength()))
		if ip.Data()+sconn.Overhead > c.MTU() {
			c.logger.Warn("capture too big segment")
			return true
		}

		id := session.ID{
			Remote: netip.AddrFrom4(hdr.DestinationAddress().As4()),
			Proto:  hdr.TransportProtocol(),
		}

		if id.Proto == header.TCPProtocolNumber {
			fatun.UpdateMSS(ip.Bytes(), -sconn.Overhead)
		}

		c.raw().uplink(c.srvCtx, ip, id)
	}
	return hit
}
