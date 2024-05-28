package udp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/crypto"
	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	MaxRecvBuff int
	TcpMssDelta int

	TLS *tls.Config

	PcapBuiltinPath string
}

type udp interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close() error
}

type Conn struct {
	config *Config
	role   role

	conn udp
	peer conn.Peer

	stack ustack.Ustack
	ep    *ustack.LinkEndpoint

	handshakedNotify chan struct{}
	handshaked       atomic.Bool    // start or final handshake
	tcp              *gonet.TCPConn // builtin tcp conn

	// crypto crypto.Crypto
}

var _ conn.Conn = (*Conn)(nil)

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

func Dial[A conn.Peer](server string, config *Config) (conn.Conn, error) {
	return DialCtx[A](context.Background(), server, config)
}

func DialCtx[A conn.Peer](ctx context.Context, server string, config *Config) (conn.Conn, error) {
	raddr, err := resolve(server, false)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: raddr.Addr().AsSlice(), Port: int(raddr.Port())})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c, err := newConn[A](conn, config)
	if err != nil {
		return nil, c.close(err)
	}

	if err := c.handshake(ctx); err != nil {
		return nil, c.close(err)
	}
	return c, nil
}

func newConn[P conn.Peer](conn udp, config *Config) (*Conn, error) {
	var c = &Conn{
		config: config,
		peer:   *(new(P)),
		role:   client,

		conn: conn,

		handshakedNotify: make(chan struct{}),
	}
	var err error

	laddr, raddr := c.LocalAddr(), c.RemoteAddr()
	// todo: fix mtu
	c.stack, err = ustack.NewUstack(link.NewList(8, 1400), laddr.Addr())
	if err != nil {
		return nil, c.close(err)
	}
	if config.PcapBuiltinPath != "" {
		c.stack = ustack.MustWrapPcap(c.stack, config.PcapBuiltinPath)
	}
	c.ep, err = ustack.NewLinkEndpoint(c.stack, laddr.Port(), raddr)
	if err != nil {
		return nil, c.close(err)
	}

	go c.outboundService()
	return c, nil
}

func (c *Conn) handshake(ctx context.Context) (err error) {
	if !c.handshaked.CompareAndSwap(false, true) {
		<-c.handshakedNotify
		return nil
	}
	go c.handshakeInboundService()

	c.tcp, err = gonet.DialTCPWithBind(
		ctx, c.stack,
		c.LocalAddr(), c.RemoteAddr(),
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	var key crypto.Key
	if c.role.Client() {
		// key, err = c.config.Handshake.Client(ctx, c.tcp)
	} else {
		// key, err = c.config.Handshake.Server(ctx, c.tcp)
	}
	if err != nil {
		return c.close(err)
	}
	if key != (crypto.Key{}) {
		// c.crypto, err = crypto.NewTCP(key, 0) // todo： udp
		// if err != nil {
		// 	return c.close(err)
		// }
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
			err := c.Recv(peer, tcp.Sets(0, 0xffff))
			if err != nil {
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

func (c *Conn) outboundService() error {
	var (
		tcp     = packet.Make(c.config.MaxRecvBuff)
		builtin = c.peer.Builtin()
	)

	for {
		err := c.ep.Outbound(context.Background(), tcp.Sets(64, 0xffff))
		if err != nil {
			return c.close(err)
		}

		err = c.Send(builtin, tcp)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Conn) close(cause error) error {
	panic(cause)
}

func (c *Conn) BuiltinConn(ctx context.Context) (tcp net.Conn, err error) {
	if err := c.handshake(ctx); err != nil {
		return nil, c.close(err)
	}
	return c.tcp, nil
}
func (c *Conn) Recv(peer conn.Peer, pkt *packet.Packet) (err error) {
	if err := c.handshake(context.Background()); err != nil {
		return c.close(err)
	}

	head, data := pkt.Head(), pkt.Data()
	for {
		n, err := c.conn.Read(pkt.Sets(head, data).Bytes())
		if err != nil {
			return c.close(err)
		}
		pkt.SetData(n)
		// todo: 校验包完整性

		// if c.crypto != nil {
		// 	if err = c.crypto.Decrypt(pkt); err != nil {
		// 		fmt.Println(err)
		// 		continue
		// 	}
		// }

		if err := peer.Decode(pkt); err != nil {
			return c.close(err)
		}

		if peer.IsBuiltin() {
			c.ep.Inbound(pkt)
		} else {
			return nil
		}
	}
}
func (c *Conn) Send(atter conn.Peer, pkt *packet.Packet) (err error) {
	if err := c.handshake(context.Background()); err != nil {
		return c.close(err)
	}

	if err = atter.Encode(pkt); err != nil {
		return c.close(err)
	}

	// if c.crypto != nil {
	// 	c.crypto.Encrypt(pkt)
	// }

	_, err = c.conn.Write(pkt.Bytes())
	if err != nil {
		return c.close(err)
	}
	return nil
}

func (c *Conn) LocalAddr() netip.AddrPort {
	return netip.MustParseAddrPort(c.conn.LocalAddr().String())
}
func (c *Conn) RemoteAddr() netip.AddrPort {
	return netip.MustParseAddrPort(c.conn.RemoteAddr().String())
}
func (c *Conn) Close() error { return c.close(nil) }

func resolve(addr string, local bool) (netip.AddrPort, error) {
	if taddr, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return netip.AddrPort{}, errors.WithStack(err)
	} else {
		if taddr.Port == 0 {
			taddr.Port = 443
		}
		if len(taddr.IP) == 0 || taddr.IP.IsUnspecified() {
			if local {
				s, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
				if err != nil {
					return netip.AddrPort{}, errors.WithStack(err)
				}
				defer s.Close()
				taddr.IP = s.LocalAddr().(*net.UDPAddr).IP
			} else {
				return netip.AddrPort{}, errors.Errorf("server address %s require ip or domain", addr)
			}
		}
		return netip.MustParseAddrPort(taddr.String()), nil
	}
}
