package sconn

import (
	"context"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/faketcp"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type tcpFactory interface {
	factory(ctx context.Context) (*gonet.TCPConn, error)
	Close() error
}

type clientFactory struct {
	local, remote netip.AddrPort
	stack         ustack.Ustack
}

func (c *clientFactory) factory(ctx context.Context) (*gonet.TCPConn, error) {
	tcp, err := gonet.DialTCPWithBind(
		ctx, c.stack,
		c.local, c.remote,
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return tcp, nil
}

func (c *clientFactory) Close() error { return c.stack.Close() }

type serverFactory struct {
	l      *gonet.TCPListener
	remote netip.AddrPort
}

func (s *serverFactory) factory(ctx context.Context) (*gonet.TCPConn, error) {
	tcp, err := s.l.AcceptBy(ctx, s.remote)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return tcp, nil
}

func (s *serverFactory) Close() error { return nil }

/*
	旧的握手状态转移管理：
		server:
			在握手完成后进入prepare阶段
			等待接收到第一个fake包， 初始化faketcp
			之后处理第一个fake包，发送的也是fake包


		根本在于等待第一个fake包

		这里面有状态转移，需要区分CS:
		cliet-up --> server-up --> server-down --> client-down

	握手流程（设计不要区分CS的）：
		handshake1完成后，（handshak2阶段）等待对方握手完成，期间将不会主动发送数据包。判定对方握手完成的依据是我方
		在握手期间发送的数据包全部被对方收到，及WaitBeforeDataTransmitted。
		a. 对于outboundService，在handshak2完成后，发送的是control-segment包, 而不是原始的tcp数据包。
		b. 对于handshakeInboundService，在handshak2完成后，才能退出。如果其运行时收到segment包，要是此时hanshake1
			已经完成，应该尝试解包，解包结果如果是control包，必须inbound；否则应该暂存。
		c. 对于Recv，如收到非segment包，应该忽略。

		注：handshakeInboundService收到segment包，Recv收到非segment包的情况都是边界竞争导致的，不会有太多数据包处于这种状态。

*/

func (c *Conn) handshake(ctx context.Context) (err error) {
	if !c.state.CompareAndSwap(initial, handshake1) {
		c.handshakedNotify.Wait() // handshake started, wait finish
		return nil
	}

	inctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.handshakeInboundService(inctx)

	tcp, err := c.factory.factory(ctx)
	if err != nil {
		return err
	}

	var key crypto.Key
	if c.role == server {
		if err := c.cfg.PrevPackets.Server(ctx, tcp); err != nil {
			return err
		}
		if key, err = c.cfg.SwapKey.Server(ctx, tcp); err != nil {
			return err
		}
	} else if c.role == client {
		if err := c.cfg.PrevPackets.Client(ctx, tcp); err != nil {
			return err
		}
		if key, err = c.cfg.SwapKey.Client(ctx, tcp); err != nil {
			return err
		}
	} else {
		return errors.Errorf("sconn invalid role %d", c.role)
	}
	if cpt, err := crypto.NewTCP(key, c.pseudoSum1); err != nil {
		return err
	} else {
		c.fake = faketcp.New(
			c.raw.LocalAddr().Port(),
			c.raw.RemoteAddr().Port(),
			faketcp.Crypto(cpt),
		)
	}
	c.state.CompareAndSwap(handshake1, handshake2)

	// wait before writen data be recved by peer.
	if err := tcp.WaitBeforeDataTransmitted(ctx); err != nil {
		return err
	}
	c.fake.InitSeqAck(c.seq, c.ack)

	c.handshakedTime = time.Now()
	c.tcp = tcp
	c.state.CompareAndSwap(handshake2, transmit)
	c.handshakedNotify.Done()
	return nil
}

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

			if c.state.Load() >= handshake2 &&
				c.fake.DetachRecv(seg) == nil &&
				session.Decode(seg) == session.CtrSessID {

				c.ep.Inbound(seg)
			} else {
				c.handshakeRecvSegs.pop(pkt)
				// todo：log
			}

			if c.state.Load() == transmit {
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

// heap simple heap buff, only support concurrent pop,
// and not support cross pop/put opeate.
type heap struct {
	data [heapsize]*packet.Packet // desc operate
	idx  atomic.Int32
}

const heapsize = 8

func (h *heap) put(pkt *packet.Packet) bool {
	if h.data[heapsize-1] != nil {
		return false
	}

	for i := 0; i < heapsize; i++ {
		if h.data[i] == nil {
			h.data[i] = pkt.Clone()
			return true
		}
	}
	return false
}

func (h *heap) pop(pkt *packet.Packet) bool {
	idx := h.idx.Add(1) - 1
	if idx >= heapsize {
		h.idx.Store(heapsize) // avoid h.idx inc overflow
		return false
	}

	if h.data[idx] != nil {
		pkt.SetData(0).Append(h.data[idx].Bytes())
		return true
	}
	return false
}
