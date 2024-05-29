package udp

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net/netip"
	"time"

	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type role uint8

const (
	client role = 1
	server role = 2
)

func (r role) Client() bool { return r == client }
func (r role) Server() bool { return r == server }
func (r role) String() string {
	switch r {
	case client:
		return "client"
	case server:
		return "server"
	default:
		return fmt.Sprintf("invalid fatcp role %d", r)
	}
}

func (c *Conn) clientFactory(ctx context.Context, remote netip.AddrPort) (*gonet.TCPConn, error) {
	return gonet.DialTCPWithBind(
		ctx, c.ep.Stack(),
		c.LocalAddr(), remote,
		header.IPv4ProtocolNumber,
	)
}

func (c *Conn) handshake(ctx context.Context) (err error) {
	if !c.handshaked.CompareAndSwap(false, true) {
		<-c.handshakedNotify
		return nil
	}
	retch := make(chan struct{})
	defer func() { <-retch }()
	go c.handshakeInboundService(retch)

	tcp, err := c.tcpFactory(ctx, c.RemoteAddr())
	if err != nil {
		return errors.WithStack(err)
	}
	stop := context.AfterFunc(ctx, func() { tcp.SetDeadline(time.Now()) })
	defer stop()

	if c.config.TLS != nil {
		var tconn *tls.Conn
		if c.role.Client() {
			tconn = tls.Client(tcp, c.config.TLS)
		} else {
			tconn = tls.Server(tcp, c.config.TLS)
		}
		if err := tconn.HandshakeContext(ctx); err != nil {
			return errors.WithStack(err)
		}

		var key Key
		if c.role.Server() {
			if n, err := rand.Read(key[:]); err != nil {
				return errors.WithStack(err)
			} else if n != len(key) {
				return errors.Errorf("crypto rand too small %d", n)
			}

			if _, err = tconn.Write(key[:]); err != nil {
				return errors.WithStack(err)
			}
		} else {
			if _, err := io.ReadFull(tconn, key[:]); err != nil {
				return errors.WithStack(err)
			}
		}
		c.crypto, err = NewCrypto(key, c.peer.Overhead())
		if err != nil {
			return errors.WithStack(err)
		}

		c.builtin = tconn
	} else {
		c.builtin = tcp
	}

	close(c.handshakedNotify)
	return nil
}
func (c *Conn) handshakeInboundService(retch chan struct{}) (_ error) {
	var (
		tcp  = packet.Make(c.config.MaxRecvBuff)
		peer = c.peer.Builtin().Reset(0, netip.IPv4Unspecified())
	)
	defer func() { close(retch) }()

	for {
		select {
		case <-c.handshakedNotify:
			return nil
		default:
			n, err := c.conn.Read(tcp.Sets(64, 0xffff).Bytes())
			if err != nil {
				return c.close(err)
			}
			tcp.SetData(n)

			if err := peer.Decode(tcp); err != nil {
				return c.close(err)
			}

			if peer.IsBuiltin() {
				c.inboundBuitinPacket(tcp)
			} else {
				select {
				case c.handshakeRecvedPackets <- tcp.AttachN(c.peer.Overhead()).Clone():
				default:
					// todo: log
				}
			}
		}
	}
}

func (c *Conn) inboundBuitinPacket(tcp *packet.Packet) {
	// if the data packet passes through the NAT gateway, on handshake
	// step, the client port will be change automatically, after handshake, need manually
	// change client port for builtin tcp packet.
	if c.role.Client() {
		header.TCP(tcp.Bytes()).SetDestinationPortWithChecksumUpdate(c.natPort)
	} else {
		header.TCP(tcp.Bytes()).SetSourcePortWithChecksumUpdate(c.natPort)
	}

	if debug.Debug() {
		hdr := header.TCP(tcp.Bytes())
		require.Equal(test.T(), c.LocalAddr().Port(), hdr.DestinationPort())
		require.Equal(test.T(), c.RemoteAddr().Port(), hdr.SourcePort())
	}
	c.ep.Inbound(tcp)
}
