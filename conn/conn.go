package conn

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	MaxRecvBuff int

	TLS *tls.Config

	PcapBuiltinPath string
}

type Conn interface {

	// BuiltinConn get builtin stream connect, require Recv be called async.
	BuiltinConn(ctx context.Context) (conn net.Conn, err error)

	Recv(peer Peer, payload *packet.Packet) (err error)
	Send(peer Peer, payload *packet.Packet) (err error)

	LocalAddr() netip.AddrPort
	RemoteAddr() netip.AddrPort
	Close() error
}

type conn struct {
	config  *Config
	role    role
	peer    Peer
	natPort uint16

	conn net.Conn

	// stack ustack.Ustack
	ep         *ustack.LinkEndpoint
	tcpFactory factory

	handshakedNotify       chan struct{}
	handshaked             atomic.Bool // start or final handshake
	builtin                net.Conn    // builtin tcp conn
	handshakeRecvedPackets chan *packet.Packet

	crypto *crypto

	closeErr errorx.CloseErr
}

func NewConn[P Peer](dgramConn net.Conn, config *Config) (Conn, error) {
	var laddr = netip.MustParseAddrPort(dgramConn.LocalAddr().String())
	var raddr = netip.MustParseAddrPort(dgramConn.RemoteAddr().String())

	stack, err := ustack.NewUstack(link.NewList(8, 1320), laddr.Addr()) // todo: fix mtu
	if err != nil {
		return nil, err
	}
	if config.PcapBuiltinPath != "" {
		stack = ustack.MustWrapPcap(stack, config.PcapBuiltinPath)
	}
	ep, err := ustack.NewLinkEndpoint(stack, laddr.Port(), raddr)
	if err != nil {
		return nil, err
	}

	return newConn(dgramConn, *new(P), client, ep, nil, config)
}

func newConn(dgramConn net.Conn, peer Peer, role role, ep *ustack.LinkEndpoint, fact factory, config *Config) (*conn, error) {
	var c = &conn{
		config: config,
		role:   role,
		peer:   peer,

		conn: dgramConn,

		ep: ep,

		handshakedNotify:       make(chan struct{}),
		handshakeRecvedPackets: make(chan *packet.Packet, 8),
	}
	if fact == nil {
		c.tcpFactory = c.clientFactory
	} else {
		c.tcpFactory = fact
	}
	if role.Client() {
		c.natPort = c.LocalAddr().Port()
	} else {
		c.natPort = c.RemoteAddr().Port()
	}

	go c.outboundService()
	return c, nil
}

func (c *conn) close(cause error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.builtin != nil {
			errs = append(errs, c.builtin.Close())
		}
		if c.ep != nil {
			errs = append(errs, c.ep.Close())
			// todo: wait
		}
		if c.conn != nil {
			errs = append(errs, c.conn.Close())
		}
		return errs
	})
}

func (c *conn) BuiltinConn(ctx context.Context) (conn net.Conn, err error) {
	if err := c.handshake(ctx); err != nil {
		return nil, c.close(err)
	}
	return c.builtin, nil
}

func (c *conn) recv(pkt *packet.Packet) (err error) {
	select {
	case p := <-c.handshakeRecvedPackets:
		n := copy(pkt.Bytes(), p.Bytes())
		pkt.SetData(n)
		if n != p.Data() {
			return errorx.ShortBuff(p.Data(), pkt.Data())
		}
	default:
		n, err := c.conn.Read(pkt.Bytes())
		if err != nil {
			return err
		}
		pkt.SetData(n)
	}
	return nil
}

func (c *conn) Recv(peer Peer, pkt *packet.Packet) (err error) {
	if err := c.handshake(context.Background()); err != nil {
		return c.close(err)
	}

	head, data := pkt.Head(), pkt.Data()
	for {
		err := c.recv(pkt.Sets(head, data))
		if err != nil {
			return c.close(err)
		}

		if err := peer.Decode(pkt); err != nil {
			return err
		}

		if peer.IsBuiltin() {
			c.inboundBuitinPacket(pkt)
		} else {
			if c.crypto != nil {
				err = c.crypto.decrypt(pkt.AttachN(c.peer.Overhead()))
				if err != nil {
					return err
				}
				pkt.DetachN(c.peer.Overhead())
			}
			return nil
		}
	}
}
func (c *conn) Send(peer Peer, pkt *packet.Packet) (err error) {
	if err := c.handshake(context.Background()); err != nil {
		return c.close(err)
	}

	if err = peer.Encode(pkt); err != nil {
		return c.close(err)
	}

	if !peer.IsBuiltin() && c.crypto != nil {
		c.crypto.encrypt(pkt)
	}

	_, err = c.conn.Write(pkt.Bytes())
	if err != nil {
		return c.close(err)
	}
	return nil
}

func (c *conn) LocalAddr() netip.AddrPort {
	return netip.MustParseAddrPort(c.conn.LocalAddr().String())
}
func (c *conn) RemoteAddr() netip.AddrPort {
	return netip.MustParseAddrPort(c.conn.RemoteAddr().String())
}
func (c *conn) Close() error { return c.close(nil) }

func (c *conn) outboundService() error {
	var (
		tcp     = packet.Make(c.config.MaxRecvBuff)
		builtin = c.peer.Builtin()
	)

	for {
		err := c.ep.Outbound(context.Background(), tcp.Sets(64, 0xffff))
		if err != nil {
			return c.close(err)
		}

		if err = builtin.Encode(tcp); err != nil {
			return c.close(err)
		}

		_, err = c.conn.Write(tcp.Bytes())
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *conn) clientFactory(ctx context.Context, remote netip.AddrPort) (*gonet.TCPConn, error) {
	return gonet.DialTCPWithBind(
		ctx, c.ep.Stack(),
		c.LocalAddr(), remote,
		header.IPv4ProtocolNumber,
	)
}

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

type factory func(ctx context.Context, remote netip.AddrPort) (*gonet.TCPConn, error)

func (c *conn) handshake(ctx context.Context) (err error) {
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

		var key key
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
		c.crypto, err = newCrypto(key, c.peer.Overhead())
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
func (c *conn) handshakeInboundService(retch chan struct{}) (_ error) {
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

func (c *conn) inboundBuitinPacket(tcp *packet.Packet) {
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
