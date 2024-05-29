package udp

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/udp/audp"
	"github.com/lysShub/fatun/ustack"
	"github.com/lysShub/fatun/ustack/gonet"
	"github.com/lysShub/fatun/ustack/link"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Config struct {
	MaxRecvBuff int
	TcpMssDelta int

	TLS *tls.Config

	PcapBuiltinPath string
}

type Conn struct {
	config *Config
	role   role

	conn audp.Conn
	peer conn.Peer

	stack ustack.Ustack
	ep    *ustack.LinkEndpoint

	handshakedNotify chan struct{}
	handshaked       atomic.Bool    // start or final handshake
	builtin          *gonet.TCPConn // builtin tcp conn

	crypto *crypto

	closeErr errorx.CloseErr
}

var _ conn.Conn = (*Conn)(nil)

func Dial[P conn.Peer](server string, config *Config) (conn.Conn, error) {
	return DialCtx[P](context.Background(), server, config)
}

func DialCtx[P conn.Peer](ctx context.Context, server string, config *Config) (conn.Conn, error) {
	raddr, err := resolve(server, false)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: raddr.Addr().AsSlice(), Port: int(raddr.Port())})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c, err := newConn(conn, *(new(P)), client, config)
	if err != nil {
		return nil, c.close(err)
	}

	if err := c.handshake(ctx); err != nil {
		return nil, c.close(err)
	}
	return c, nil
}

func newConn(conn audp.Conn, peer conn.Peer, role role, config *Config) (*Conn, error) {
	var c = &Conn{
		config: config,
		peer:   peer,
		role:   role,

		conn: conn,

		handshakedNotify: make(chan struct{}),
	}
	var err error

	laddr, raddr := c.LocalAddr(), c.RemoteAddr()
	c.stack, err = ustack.NewUstack(link.NewList(8, 512), laddr.Addr()) // todo: fix mtu
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

func (c *Conn) close(cause error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.ep != nil {
			errs = append(errs, c.ep.Close())
			// todo: wait
		}
		if c.stack != nil {
			errs = append(errs, c.stack.Close())
		}
		if c.conn != nil {
			errs = append(errs, c.conn.Close())
		}
		return errs
	})
}

func (c *Conn) BuiltinConn(ctx context.Context) (conn net.Conn, err error) {
	if err := c.handshake(ctx); err != nil {
		return nil, c.close(err)
	}
	return c.builtin, nil
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

		if err := peer.Decode(pkt); err != nil {
			return err
		}

		if peer.IsBuiltin() {
			c.ep.Inbound(pkt)
		} else {
			if c.crypto != nil {
				err = c.crypto.Decrypt(pkt.AttachN(c.peer.Overhead()))
				if err != nil {
					return err
				}
				pkt.DetachN(c.peer.Overhead())
			}
			return nil
		}
	}
}
func (c *Conn) Send(peer conn.Peer, pkt *packet.Packet) (err error) {
	if err := c.handshake(context.Background()); err != nil {
		return c.close(err)
	}

	if err = peer.Encode(pkt); err != nil {
		return c.close(err)
	}

	if !peer.IsBuiltin() && c.crypto != nil {
		c.crypto.Encrypt(pkt)
	}

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

		if err = builtin.Encode(tcp); err != nil {
			return c.close(err)
		}

		_, err = c.conn.Write(tcp.Bytes())
		if err != nil {
			return c.close(err)
		}
	}
}

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
