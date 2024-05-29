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
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
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

func (c *Conn) handshake(ctx context.Context) (err error) {
	if !c.handshaked.CompareAndSwap(false, true) {
		<-c.handshakedNotify
		return nil
	}
	go c.handshakeInboundService()

	tcp, err := gonet.DialTCPWithBind(
		ctx, c.stack,
		c.LocalAddr(), c.RemoteAddr(),
		header.IPv4ProtocolNumber,
	)
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
	}

	close(c.handshakedNotify)
	return nil
}
func (c *Conn) handshakeInboundService() (_ error) {
	var (
		tcp  = packet.Make(c.config.MaxRecvBuff)
		peer = c.peer.Builtin().Reset(0, netip.IPv4Unspecified())
	)

	for {
		select {
		case <-c.handshakedNotify:
			return nil
		default:

			n, err := c.conn.Read(tcp.Sets(0, 0xffff).Bytes())
			if err != nil {
				return c.close(err)
			}
			tcp.SetData(n)

			if err := peer.Decode(tcp); err != nil {
				return c.close(err)
			}

			if peer.IsBuiltin() {
				c.ep.Inbound(tcp)
			} else {
				fmt.Println("缓存起来")
			}
		}
	}
}
