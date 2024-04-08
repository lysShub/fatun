package sconn

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"sync/atomic"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/faketcp"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/sockit/conn"
	"github.com/lysShub/sockit/packet"

	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Sconn interface {
	net.Conn // control tcp conn

	// Recv/Send segment packet
	Recv(ctx context.Context, pkt *packet.Packet) (id session.ID, err error)
	Send(ctx context.Context, pkt *packet.Packet, id session.ID) (err error)
}

// security datagram conn
type Conn struct {
	cfg  *Config
	raw  conn.RawConn
	role role

	ep       ustack.Endpoint
	net.Conn // control tcp conn

	inited     atomic.Bool
	precvRecvs atomic.Pointer[[]*packet.Packet]

	pseudoSum1 uint16           //
	seq, ack   uint32           // init seq/ack
	fake       *faketcp.FakeTCP //

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

type role string

const (
	client role = "client"
	server role = "server"
)

func newConn(raw conn.RawConn, ep ustack.Endpoint, role role, cfg *Config) (*Conn, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}

	var c = &Conn{
		cfg:  cfg,
		raw:  raw,
		role: role,

		ep: ep,
	}
	c.precvRecvs.Store(new([]*packet.Packet))
	c.pseudoSum1 = header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		tcpip.AddrFromSlice(c.raw.LocalAddr().Addr().AsSlice()),
		tcpip.AddrFromSlice(c.raw.RemoteAddr().Addr().AsSlice()),
		0,
	)
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())
	return c, nil
}

func (c *Conn) close(cause error) (err error) {
	if cause == nil {
		return *c.closeErr.Load()
	}

	if c.closeErr.CompareAndSwap(nil, &cause) {
		if c.Conn != nil {
			cause = errorx.Join(cause, c.Conn.Close())
		}
		if c.ep != nil {
			cause = errorx.Join(cause, c.ep.Close())
		}

		if c.srvCancel != nil {
			c.srvCancel()
		}

		if c.raw != nil {
			cause = errorx.Join(cause, c.raw.Close())
		}

		c.closeErr.Store(&cause)
		return cause
	} else {
		return *c.closeErr.Load()
	}
}

func (c *Conn) Overhead() int {
	n := session.Size
	return n + c.fake.Overhead()
}

func (c *Conn) handshakeClient(ctx context.Context, stack ustack.Ustack) error {
	inctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.handshakeInboundService(inctx)
	go c.outboundService()

	tcp, err := gonet.DialTCPWithBind(
		ctx, stack,
		c.raw.LocalAddr(), c.raw.RemoteAddr(),
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return c.close(err)
	}

	if err := c.handshake(ctx, tcp); err != nil {
		tcp.Close()
		return c.close(err)
	}
	return nil
}

func (c *Conn) handshakeServer(ctx context.Context, l *gonet.TCPListener) error {
	inctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.handshakeInboundService(inctx)
	go c.outboundService()

	tcp, err := l.AcceptBy(ctx, c.raw.RemoteAddr())
	if err != nil {
		return c.close(err)
	}

	if err := c.handshake(ctx, tcp); err != nil {
		tcp.Close()
		return c.close(err)
	}
	return nil
}

func (c *Conn) handshake(ctx context.Context, tcp *gonet.TCPConn) (err error) {
	var key crypto.Key
	if c.role == server {
		if err := c.cfg.PrevPackets.Server(ctx, tcp); err != nil {
			return err
		}
		if key, err = c.cfg.SwapKey.Server(ctx, tcp); err != nil {
			return err
		}
	} else {
		if err := c.cfg.PrevPackets.Client(ctx, tcp); err != nil {
			return err
		}
		if key, err = c.cfg.SwapKey.Client(ctx, tcp); err != nil {
			return err
		}
	}
	cpt, err := crypto.NewTCP(key, c.pseudoSum1)
	if err != nil {
		return err
	}

	if err := tcp.WaitSendbuffDrained(ctx); err != nil {
		return err
	}

	c.fake = faketcp.New(
		c.raw.LocalAddr().Port(),
		c.raw.RemoteAddr().Port(),
		faketcp.SeqAck(c.seq, c.ack), faketcp.Crypto(cpt),
	)
	c.inited.Store(true)

	// 当前control的mtu直接是原始mtu减去overhead, 可以确保mtu容量的
	// pkt在Send/Recv, 但这样会导致在握手阶段的数据包大小不符合期望。
	// 应该在握手完成后更改MaxPayloadSize
	// todo: should be change endpoint.sender.MaxPayloadSize
	// if err := tcp.SetSockOptInt(
	// 	tcpip.MaxSegOption, c.cfg.HandshakeMTU-c.Overhead(),
	// ); err != nil {
	// 	return c.close(err)
	// }

	c.Conn = tcp
	return nil
}

/*
	旧的管理：
		server:
			在握手完成后进入prepare阶段
			等待接收到第一个fake包， 初始化faketcp
			继续处理第一个fake包，发送的也是fake包


		根本在于等待第一个fake包

		这里面有状态转移，需要区分CS:
		cliet-up --> server-up --> server-down --> client-down

	现在尝试设计不要区分CS的：
	1. swapkey完成时，等待sendbuff清空后，inited。之后：
		outboundService发送的将是faketcp（buff清空只是代表发出去了，不代表收到，可能重传，重传的不应该fake、否则对面将无法完成swapkey，可以依据seq进行判定）。
		handshakeInboundService 将立即退出；如果在inited之前收到fake包，应该缓存（这是由于IP包乱序导致的）。
*/

func (c *Conn) handshakeInboundService(ctx context.Context) error {
	var (
		pkt = packet.Make(0, c.cfg.HandshakeMTU)
	)

	for {
		err := c.raw.Read(ctx, pkt.SetHead(0))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return c.close(err)
		}

		if debug.Debug() {
			old := pkt.Head()
			pkt.SetHead(0)
			test.ValidIP(test.T(), pkt.Bytes())
			pkt.SetHead(old)
		}

		if faketcp.Is(pkt.Bytes()) {
			if !c.inited.Load() {
				s := c.precvRecvs.Load()
				*s = append(*s, pkt.Clone())
				c.precvRecvs.Store(s)
			} else {
				return nil
			}
		} else {
			// record ack
			tcp := header.TCP(pkt.Bytes())
			// c.ack = max(c.ack, tcp.SequenceNumber())
			c.ack = max(c.ack, tcp.SequenceNumber()+uint32(len(tcp.Payload())))

			c.ep.Inbound(pkt)
		}
	}
}

func (c *Conn) outboundService() error {
	var (
		pkt = packet.Make(0, c.cfg.HandshakeMTU)
	)

	for {
		err := c.ep.Outbound(c.srvCtx, pkt.SetHead(0))
		if err != nil {
			return c.close(err)
		}

		if c.inited.Load() {
			// todo: better use tcp seq
			if debug.Debug() {
				tcp := header.TCP(pkt.Bytes())
				v := tcp.SequenceNumber() + uint32(len(tcp.Payload()))
				require.GreaterOrEqual(test.T(), v, c.seq)
			}

			err = c.Send(c.srvCtx, pkt, session.CtrSessID)
			if err != nil {
				return c.close(err)
			}
		} else {
			tcphdr := header.TCP(pkt.Bytes())
			c.seq = max(c.seq, tcphdr.SequenceNumber()+uint32(len(tcphdr.Payload())))

			err = c.raw.Write(context.Background(), pkt)
			if err != nil {
				return c.close(err)
			}

			if debug.Debug() {
				test.ValidIP(test.T(), pkt.SetHead(0).Bytes())
			}
		}
	}
}

func (c *Conn) Send(ctx context.Context, pkt *packet.Packet, id session.ID) (err error) {
	session.Encode(pkt, id)
	c.fake.AttachSend(pkt)

	if debug.Debug() {
		test.ValidTCP(test.T(), pkt.Bytes(), c.pseudoSum1)
		require.True(test.T(), faketcp.Is(pkt.Bytes()))
	}

	err = c.raw.Write(ctx, pkt)
	return err
}

func (c *Conn) recv(ctx context.Context, pkt *packet.Packet) error {
	if p := c.precvRecvs.Swap(nil); p != nil {
		if n := len(*p); n > 0 {
			e := (*p)[n-1]
			pkt.SetData(0).Append(e.Bytes())

			*p = (*p)[:n-1]
			c.precvRecvs.Store(p)
			return nil
		}
	}

	err := c.raw.Read(ctx, pkt)
	return err
}

func (c *Conn) Recv(ctx context.Context, pkt *packet.Packet) (id session.ID, err error) {
	head := pkt.Head()
	for {
		if err = c.recv(ctx, pkt.SetHead(head)); err != nil {
			return 0, err
		}

		err = c.fake.DetachRecv(pkt)
		if err != nil {
			return 0, errorx.Temporary(err)
		}

		id = session.Decode(pkt)
		if id == session.CtrSessID {
			c.ep.Inbound(pkt)
			continue
		}
		return id, nil
	}
}

func (c *Conn) LocalAddrPort() netip.AddrPort  { return c.raw.LocalAddr() }
func (c *Conn) RemoteAddrPort() netip.AddrPort { return c.raw.RemoteAddr() }
func (c *Conn) Close() error                   { return c.close(os.ErrClosed) }
