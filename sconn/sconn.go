package sconn

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/faketcp"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/sockit/conn"
	"github.com/lysShub/sockit/errorx"
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

	ep  ustack.Endpoint
	tcp net.Conn // control tcp conn

	inited     atomic.Bool
	initedTime time.Time
	precvRecvs atomic.Pointer[[]*packet.Packet]

	pseudoSum1 uint16           //
	seq, ack   uint32           // init seq/ack
	fake       *faketcp.FakeTCP //
	fakeinited atomic.Bool

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

func (c *Conn) close(cause error) error {
	if c.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if c.tcp != nil {
			// maybe closed before, ignore return error
			c.tcp.Close()

			// wait tcp close finished
			if gotcp, ok := c.tcp.(*gonet.TCPConn); ok {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				gotcp.WaitBeforeDataTransmitted(ctx)
			}
		}
		if c.ep != nil {
			if err := c.ep.Close(); err != nil {
				cause = err
			}
		}

		if c.srvCancel != nil {
			c.srvCancel()
		}

		if c.raw != nil {
			if err := c.raw.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			c.closeErr.Store(&cause)
		}
		return cause
	}
	return *c.closeErr.Load()
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

	if cpt, err := crypto.NewTCP(key, c.pseudoSum1); err != nil {
		return err
	} else {
		c.fake = faketcp.New(
			c.raw.LocalAddr().Port(),
			c.raw.RemoteAddr().Port(),
			faketcp.Crypto(cpt),
		)
		c.fakeinited.Store(true)
	}

	// wait before writen data be recved by peer.
	if err := tcp.WaitBeforeDataTransmitted(ctx); err != nil {
		return err
	}
	c.fake.InitSeqAck(c.seq, c.ack)

	c.inited.Store(true)
	c.initedTime = time.Now()

	c.tcp = tcp
	return nil
}

/*
	旧的握手状态转移管理：
		server:
			在握手完成后进入prepare阶段
			等待接收到第一个fake包， 初始化faketcp
			之后处理第一个fake包，发送的也是fake包


		根本在于等待第一个fake包

		这里面有状态转移，需要区分CS:
		cliet-up --> server-up --> server-down --> client-down

	新的（设计不要区分CS的）：
		swapkey完成时，不主动发送任何数据包，等待发送的数据被对方接收到，然后inited。之后：
			1.outboundService发送的将是faketcp。
			2.handshakeInboundService 将立即退出、并循环调用Recv。
				a.如果在inited之前（handshakeInboundService）收到fake包，应该缓存(×)，这是peer先完成握手导致的；
				b.如果在inited之后，（Recv）收到非fake包，因该忽略，这是peer后完成握手，重传的数据包导致的；
				a,b 两种情况都是边界竞争导致的，不会有太多的数据包处于这种状态。

		存在问题：如果一方完先成握手, 发送的数据包将是fake, 对方只会暂存此包,但是此包可能是对方期望的ack包, 也就可能导致对方
				 的WaitBeforeDataTransmitted 始终阻塞。
				解决方法：1. 使用seq   2. init之前即可解包fake
					尝试方案2, 在initfake后尝试解包, 如果是ctrid 则注入,fake需要New和SetInitSeq, 在SetInitSeq之前不能发送数据包


		// （buff清空只是代表发出去了，不代表收到，可能重传，重传的不应该fake、否则对面将无法完成swapkey，可以依据seq进行判定）
*/

func (c *Conn) handshakeInboundService(ctx context.Context) error {
	var (
		pkt = packet.Make(64, c.cfg.HandshakeMTU)
	)

	for {
		err := c.raw.Read(ctx, pkt.SetHead(64))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return c.close(err)
		}

		if debug.Debug() {
			old := pkt.Head()
			pkt.SetHead(64)
			test.ValidIP(test.T(), pkt.Bytes())
			pkt.SetHead(old)
		}

		if faketcp.Is(pkt.Bytes()) {
			seg := pkt.Clone()
			if c.fakeinited.Load() &&
				c.fake.DetachRecv(seg) == nil &&
				session.Decode(seg) == session.CtrSessID {

				c.ep.Inbound(seg)
			} else {
				s := c.precvRecvs.Load()
				*s = append(*s, pkt.Clone())
				c.precvRecvs.Store(s)
			}

			if c.inited.Load() {
				return nil
			}
		} else {
			// record ack
			tcp := header.TCP(pkt.Bytes())
			c.ack = max(c.ack, tcp.SequenceNumber()+uint32(len(tcp.Payload())))

			c.ep.Inbound(pkt)
		}
	}
}

func (c *Conn) outboundService() error {
	var (
		pkt = packet.Make(64, c.cfg.HandshakeMTU)
	)

	for {
		err := c.ep.Outbound(c.srvCtx, pkt.SetHead(64))
		if err != nil {
			return c.close(err)
		}

		if c.inited.Load() {
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
				test.ValidIP(test.T(), pkt.SetHead(64).Bytes())
			}
		}
	}
}

func (c *Conn) Overhead() int {
	n := session.Size
	return n + c.fake.Overhead()
}

func (c *Conn) TCP() net.Conn { return c.tcp }

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
		err = c.recv(ctx, pkt.SetHead(head))
		if err != nil {
			return 0, err
		}

		err = c.fake.DetachRecv(pkt)
		if err != nil {
			if time.Since(c.initedTime) < time.Second*3 {
				continue
			}
			return 0, errorx.WrapTemp(err)
		}

		id = session.Decode(pkt)
		if id == session.CtrSessID {
			c.ep.Inbound(pkt)
			continue
		}
		return id, nil
	}
}

func (c *Conn) LocalAddr() netip.AddrPort  { return c.raw.LocalAddr() }
func (c *Conn) RemoteAddr() netip.AddrPort { return c.raw.RemoteAddr() }
func (c *Conn) Close() error               { return c.close(nil) }
