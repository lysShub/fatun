package control

import (
	"net"
	"sync/atomic"

	"github.com/lysShub/itun/cctx"
	"google.golang.org/grpc/grpclog"
)

type nullWriter struct{}

func (nullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func init() {
	// todo: take over log
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(
		nullWriter{}, nullWriter{}, nullWriter{},
	))

}

type listenerWrap struct {
	ctx      cctx.CancelCtx
	accetped atomic.Bool
	conn     net.Conn
}

func newListenerWrap(ctx cctx.CancelCtx, conn net.Conn) *listenerWrap {
	return &listenerWrap{ctx: ctx, conn: conn}
}

var _ net.Listener = (*listenerWrap)(nil)

func (l *listenerWrap) Accept() (net.Conn, error) {
	select {
	case <-l.ctx.Done():
		return nil, &net.OpError{
			Op:     "accept",
			Net:    l.conn.LocalAddr().Network(),
			Source: l.conn.LocalAddr(),
			Err:    l.ctx.Err(),
		}
	default:
	}

	if l.accetped.CompareAndSwap(false, true) {
		return l.conn, nil
	} else {
		<-l.ctx.Done()
		return nil, &net.OpError{
			Op:     "accept",
			Net:    l.conn.LocalAddr().Network(),
			Source: l.conn.LocalAddr(),
			Err:    l.ctx.Err(),
		}
	}
}

func (l *listenerWrap) Close() error   { return nil }
func (l *listenerWrap) Addr() net.Addr { return l.conn.LocalAddr() }
