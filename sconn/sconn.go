package sconn

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/test"
	"github.com/lysShub/rsocket/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type role string

const (
	client role = "client"
	server role = "server"
)

// security datagram conn
type Conn struct {
	cfg  *Config
	raw  rsocket.RawConn
	role role

	pseudoSum1 uint16
	seq, ack   uint32

	fake   *faketcp.FakeTCP
	crypto *crypto.TCP

	closeErr atomic.Pointer[error]
}

func newConn(raw rsocket.RawConn, role role, cfg *Config) (*Conn, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}

	var c = &Conn{
		cfg:  cfg,
		raw:  raw,
		role: role,
	}
	c.pseudoSum1 = header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		tcpip.AddrFromSlice(c.raw.LocalAddr().Addr().AsSlice()),
		tcpip.AddrFromSlice(c.raw.RemoteAddr().Addr().AsSlice()),
		0,
	)

	return c, nil
}

func (c *Conn) close(cause error) (err error) {
	if cause == nil {
		cause = os.ErrClosed
	}

	if c.closeErr.CompareAndSwap(nil, &cause) {
		err = cause

		if c.raw != nil {
			err = errorx.Join(err,
				c.raw.Close(),
			)
		}
		return err
	} else {
		return *c.closeErr.Load()
	}
}

func (c *Conn) handshakeConnect(parentCtx context.Context, stack ustack.Ustack) error {
	ctx := cctx.WithContext(parentCtx)
	defer ctx.Cancel(nil)
	go c.inboundService(ctx, stack)
	go c.outboundService(ctx, stack)

	tcp, err := gonet.DialTCPWithBind(
		ctx, stack,
		c.raw.LocalAddr(), c.raw.RemoteAddr(),
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return c.close(errorx.Join(err, ctx.Err()))
	}

	err = c.cfg.PrevPackets.Client(ctx, tcp)
	if err != nil {
		return c.close(err)
	}

	key, err := c.cfg.SwapKey.SecretKey(ctx, tcp)
	if err != nil {
		return c.close(err)
	} else {
		c.crypto, err = crypto.NewTCP(key, c.pseudoSum1)
		if err != nil {
			return c.close(err)
		}
	}

	if err = tcp.Close(); err != nil {
		return c.close(err)
	}
	ctx.Cancel(nil)

	c.fake = faketcp.NewFakeTCP(
		c.raw.LocalAddr().Port(), c.raw.RemoteAddr().Port(),
		faketcp.InitSeqAck(c.seq, c.ack), faketcp.PseudoSum1(c.pseudoSum1), faketcp.SeqOverhead(crypto.Bytes),
	)

	return nil
}

func (c *Conn) handshakeAccept(parentCtx context.Context, stack ustack.Ustack, l *gonet.TCPListener) error {
	ctx := cctx.WithContext(parentCtx)
	defer ctx.Cancel(nil)
	go c.inboundService(ctx, stack)
	go c.outboundService(ctx, stack)

	tcp, err := l.AcceptBy(ctx, c.raw.RemoteAddr())
	if err != nil {
		return c.close(err)
	}

	if err = c.cfg.PrevPackets.Server(ctx, tcp); err != nil {
		return c.close(err)
	}
	if key, err := c.cfg.SwapKey.SecretKey(ctx, tcp); err != nil {
		return c.close(err)
	} else {
		c.crypto, err = crypto.NewTCP(key, c.pseudoSum1)
		if err != nil {
			return c.close(err)
		}
	}

	// wait tcp close
	if err = waitClose(tcp); err != nil {
		return c.close(err)
	}
	ctx.Cancel(nil)

	// todo: NewFakeTCP not need calc csum
	c.fake = faketcp.NewFakeTCP(
		c.raw.LocalAddr().Port(),
		c.raw.RemoteAddr().Port(),
		faketcp.InitSeqAck(c.seq, c.ack), faketcp.PseudoSum1(c.pseudoSum1), faketcp.SeqOverhead(crypto.Bytes),
	)

	return nil
}

func waitClose(conn net.Conn) error {
	var b = make([]byte, 1)
	n, err := conn.Read(b)
	if n > 0 {
		return errors.New("peer not close")
	} else {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return errors.WithStack(err)
	}
}

func (c *Conn) inboundService(ctx cctx.CancelCtx, stack ustack.Ustack) {
	var (
		ip  = rsocket.NewPacket(0, c.cfg.MTU)
		ret = false
	)

	for !ret {
		ip.Sets(0, c.cfg.MTU)

		err := c.raw.ReadCtx(ctx, ip)
		if err != nil {
			ctx.Cancel(err)
			break
		}

		// record ack
		tcphdr := header.TCP(ip.Data())
		c.ack = max(c.ack, tcphdr.SequenceNumber())

		// avoid read segment packet
		ret = link.IsFakeFIN(tcphdr)

		// recover to ip packet
		ip.SetHead(0)
		if debug.Debug() {
			test.ValidIP(test.T(), ip.Data())
		}

		stack.Inbound(ip)
	}
}

func (c *Conn) outboundService(ctx cctx.CancelCtx, stack ustack.Ustack) {
	var (
		ip  = rsocket.NewPacket(0, c.cfg.MTU)
		dst = c.raw.RemoteAddr()
	)

	for {
		ip.SetHead(0)
		err := stack.OutboundBy(ctx, dst, ip)
		if err != nil {
			ctx.Cancel(err)
			break
		}

		tcphdr := header.TCP(ip.Data())
		c.seq = max(c.seq, tcphdr.SequenceNumber()+uint32(len(tcphdr.Payload())))

		// recover to ip packet
		ip.SetHead(0)
		if debug.Debug() {
			test.ValidIP(test.T(), ip.Data())
		}

		_, err = c.raw.Write(ip.Data())
		if err != nil {
			ctx.Cancel(err)
			break
		}
	}
}

func (c *Conn) Send(ctx context.Context, pkt *rsocket.Packet, id session.ID) (err error) {
	session.SetID(pkt, id)
	c.fake.SendAttach(pkt)

	c.crypto.Encrypt(pkt)
	if debug.Debug() {
		test.ValidTCP(test.T(), pkt.Data(), c.pseudoSum1)
		require.True(test.T(), faketcp.IsFakeTCP(pkt.Data()))
	}

	err = c.raw.WriteCtx(ctx, pkt)
	return err
}

func (c *Conn) Recv(ctx context.Context, pkt *rsocket.Packet) (id session.ID, err error) {
	err = c.raw.ReadCtx(ctx, pkt)
	if err != nil {
		return 0, err
	}

	err = c.crypto.Decrypt(pkt)
	if err != nil {
		return 0, errorx.Temporary(err)
	}

	c.fake.RecvStrip(pkt)

	return session.GetID(pkt), nil
}

func (c *Conn) LocalAddr() netip.AddrPort {
	return c.raw.LocalAddr()
}

func (c *Conn) RemoteAddr() netip.AddrPort {
	return c.raw.RemoteAddr()
}

func (c *Conn) Close() error {
	return c.close(os.ErrClosed)
}
