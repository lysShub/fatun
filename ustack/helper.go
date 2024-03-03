package ustack

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/relraw"
	pkge "github.com/pkg/errors"
)

type TCP struct {
	net.Conn

	ctx cctx.CancelCtx

	link  link.LinkEndpoint
	stack *Ustack

	closed   atomic.Bool
	closedCh chan struct{} // ensure in/out bound goroutine is returned
}

func AcceptTCP(paretnCtx context.Context, raw *itun.RawConn, link link.LinkEndpoint, handshakeTimeout time.Duration) (*TCP, error) {
	ctx := cctx.WithContext(paretnCtx)
	var t = newTcpStack(ctx, link, raw, "server")

	t.Conn = t.stack.Accept(ctx, handshakeTimeout)
	if err := ctx.Err(); err != nil {
		ctx.Cancel(err)
		return nil, err
	}

	return t, nil
}

func ConnectTCP(paretnCtx context.Context, raw *itun.RawConn, link link.LinkEndpoint, handshakeTimeout time.Duration) (*TCP, error) {
	ctx := cctx.WithContext(paretnCtx)
	var t = newTcpStack(ctx, link, raw, "client")
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	t.Conn = t.stack.Connect(ctx, handshakeTimeout)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return t, nil
}

func newTcpStack(ctx cctx.CancelCtx, link link.LinkEndpoint, raw *itun.RawConn, id string) *TCP {
	var t = &TCP{
		ctx:      ctx,
		link:     link,
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

func (t *TCP) Close() (err error) {
	if !t.closed.CompareAndSwap(false, true) {
		return nil // closed
	}

	if e := t.ctx.Err(); e != nil {
		err = e // cancel/exceed
	} else {

		err = t.Conn.Close()
		select {
		case <-t.link.FinRstFlag():
		case <-time.After(time.Second * 3):
			err = errors.Join(err, link.ErrTCPCloseTimeout{})
		}

		// must cancel after send SYN/RST tcp packet
		t.ctx.Cancel(nil)
		for i := 0; i < 2; i++ {
			select {
			case <-t.closedCh:
			case <-time.After(time.Second * 3):
				err = errors.Join(err,
					pkge.Errorf("user stack close timeout"),
				)
			}
		}
		close(t.closedCh)
	}

	t.stack.Destroy()
	t.link.Close()
	return err
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
