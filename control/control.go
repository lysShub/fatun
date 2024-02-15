package control

import (
	"context"
	"net"
	"os"
	"sync/atomic"

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
	ctx      context.Context
	accetped atomic.Bool
	conn     net.Conn
}

func newListenerWrap(ctx context.Context, conn net.Conn) *listenerWrap {
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
			Err:    os.ErrClosed,
		}
	default:
		if l.accetped.CompareAndSwap(false, true) {
			return l.conn, nil
		} else {
			<-l.ctx.Done()
			return nil, &net.OpError{
				Op:     "accept",
				Net:    l.conn.LocalAddr().Network(),
				Source: l.conn.LocalAddr(),
				Err:    os.ErrClosed,
			}
		}
	}
}

func (l *listenerWrap) Close() error   { return nil }
func (l *listenerWrap) Addr() net.Addr { return l.conn.LocalAddr() }
