package ustack

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/ustack/link/nofin"
	"github.com/lysShub/relraw"
)

type TCP struct {
	net.Conn

	ctx cctx.CancelCtx

	link  *nofin.Endpoint
	stack *Ustack

	closed   atomic.Bool
	closedCh chan struct{}
}

func AcceptNoFinTCP(paretnCtx context.Context, raw *itun.RawConn, handshakeTimeout time.Duration) (*TCP, error) {
	ctx := cctx.WithContext(paretnCtx)
	var t = newTcpStack(ctx, raw, "server")

	t.Conn = t.stack.Accept(ctx, handshakeTimeout)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil, err
	}

	return t, nil
}

func ConnectNoFinTCP(paretnCtx context.Context, raw *itun.RawConn, handshakeTimeout time.Duration) (*TCP, error) {
	ctx := cctx.WithContext(paretnCtx)
	var t = newTcpStack(ctx, raw, "client")
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	t.Conn = t.stack.Connect(ctx, handshakeTimeout)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return t, nil
}

func newTcpStack(ctx cctx.CancelCtx, raw *itun.RawConn, id string) *TCP {
	var t = &TCP{
		ctx:  ctx,
		link: nofin.New(8, uint32(raw.MTU())),
		// link:     channel.New(8, 1536, ""),
		closedCh: make(chan struct{}, 2),
	}

	var err error
	t.stack, err = NewUstack(
		t.link,
		raw.LocalAddrPort(), raw.RemoteAddrPort(),
	)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}
	t.stack.SetID(id)

	go t.inboundService(raw)
	go t.outboundService(raw)
	return t
}

func (t *TCP) destroy() {
	t.stack.Destroy()
	t.link.Close()
}

func (t *TCP) Close() error {
	if !t.closed.CompareAndSwap(false, true) {
		return nil // closed
	}
	defer t.destroy()

	err := t.Conn.Close()

	if e := t.ctx.Err(); e != nil {
		err = errors.Join(err, e) // cancel/exceed
	} else {
		err = errors.Join(err, WaitTCPClose(t.Conn))

		t.ctx.Cancel(nil)
	}

	select {
	case <-t.closedCh:
	case <-time.After(time.Second * 3):
		return errors.Join(
			err,
			fmt.Errorf("user stack close timeout"),
		)
	}

	select {
	case <-t.closedCh:
	case <-time.After(time.Second * 3):
		return errors.Join(
			err,
			fmt.Errorf("user stack close timeout"),
		)
	}

	close(t.closedCh)
	return err
}

func (t *TCP) SeqAck() (seq, ack uint32) {
	return t.link.SeqAck()
}

func (t *TCP) inboundService(raw *itun.RawConn) {
	mtu := raw.MTU()
	b := make([]byte, mtu)
	var p = relraw.ToPacket(0, b)

	for {
		p.Sets(0, mtu)

		if err := raw.ReadCtx(t.ctx, p); err != nil {
			t.fail(err)
			return
		}

		// recover tcp to ip
		p.SetHead(0)

		if err := t.stack.InboundRaw(p); err != nil {
			t.fail(err)
			return
		}
	}
}

func (t *TCP) outboundService(raw *itun.RawConn) {
	mtu := raw.MTU()
	var ip = relraw.NewPacket(0, mtu)

	for {
		ip.Sets(0, mtu)

		err := t.stack.Outbound(t.ctx, ip)
		if err != nil {
			t.fail(err)
			return
		}

		if _, err := raw.Write(ip.Data()); err != nil {
			t.fail(err)
			return
		}
	}
}

func (t *TCP) fail(err error) {
	if err == nil {
		return
	}

	t.ctx.Cancel(err)
	select {
	case t.closedCh <- struct{}{}:
	default:
	}
}
