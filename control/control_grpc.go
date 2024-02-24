package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control/internal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

func newGrpcClient(parentCtx cctx.CancelCtx, ctr *Controller, conn net.Conn, timeout time.Duration) *grpcClient {
	ctx := cctx.WithTimeout(parentCtx, timeout)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return conn, nil
		}),
	}

	gconn, err := grpc.DialContext(ctx, "", opts...)
	if err != nil {
		parentCtx.Cancel(err)
		return nil
	}

	return &grpcClient{
		ctr:   ctr,
		tcp:   conn,
		gconn: gconn,
		cc:    internal.NewControlClient(gconn),
		ctx:   parentCtx,
	}
}

type grpcClient struct {
	ctr   *Controller
	tcp   net.Conn
	gconn *grpc.ClientConn
	cc    internal.ControlClient
	ctx   cctx.CancelCtx
}

var _ Client = (*grpcClient)(nil)

func (c *grpcClient) Close() error {
	err := errors.Join(
		c.gconn.Close(),
		c.tcp.Close(),
		c.ctr.close(),
	)
	c.ctx.Cancel(nil)
	return err
}

func (c *grpcClient) IPv6(ctx context.Context) (bool, error) {
	r, err := c.cc.IPv6(ctx, &internal.Null{})
	if err != nil {
		return false, err
	}
	return r.Val, nil
}

func (c *grpcClient) EndConfig(ctx context.Context) error {
	_, err := c.cc.EndConfig(ctx, &internal.Null{})
	return err
}

func (c *grpcClient) AddTCP(ctx context.Context, addr netip.AddrPort) (uint16, error) {
	s, err := c.cc.AddTCP(ctx, &internal.String{Str: addr.String()})
	return uint16(s.ID), errors.Join(err, internal.Ge(s.Err))
}

func (c *grpcClient) DelTCP(ctx context.Context, id uint16) error {
	s, err := c.cc.DelTCP(ctx, &internal.SessionID{ID: uint32(id)})
	return errors.Join(err, internal.Ge(s))
}

func (c *grpcClient) AddUDP(ctx context.Context, addr netip.AddrPort) (uint16, error) {
	s, err := c.cc.AddUDP(ctx, &internal.String{Str: addr.String()})
	return uint16(s.ID), errors.Join(err, internal.Ge(s.Err))
}

func (c *grpcClient) DelUDP(ctx context.Context, id uint16) error {
	s, err := c.cc.DelUDP(ctx, &internal.SessionID{ID: uint32(id)})
	return errors.Join(err, internal.Ge(s))
}

func (c *grpcClient) PackLoss(ctx context.Context) (float32, error) {
	r, err := c.cc.PackLoss(ctx, &internal.Null{})
	if err != nil {
		return 0, err
	}
	return r.Val, err
}

func (c *grpcClient) Ping(ctx context.Context) error {
	_, err := c.cc.Ping(ctx, &internal.Null{})
	return err
}

type grpcServer struct {
	internal.UnimplementedControlServer

	ctx cctx.CancelCtx

	hdr SrvHandler

	ctr *Controller

	srv *grpc.Server
}

func serveGrpc(ctx cctx.CancelCtx, ctr *Controller, conn net.Conn, hdr SrvHandler) {
	var s = &grpcServer{
		ctx: ctx,
		hdr: hdr,

		ctr: ctr,
		srv: grpc.NewServer(),
	}
	internal.RegisterControlServer(s.srv, s)
	tcp := wrapConn(conn)

	var serveCh = make(chan struct{})
	var serveErr error
	go func() {
		serveErr = s.srv.Serve(newListenerWrap(s.ctx, tcp))
		close(serveCh)
	}()

	var err error
	select {
	case <-serveCh:
		err = net.ErrClosed
	case <-tcp.Done():
		err = serveErr
	}

	err = errors.Join(
		err,
		s.Close(),
	)

	ctxp := ctx.Ptr()
	fmt.Println(ctxp)

	s.ctx.Cancel(err)
}

func (s *grpcServer) Close() error {
	s.srv.Stop()
	return s.ctr.close()
}

var _ internal.ControlServer = (*grpcServer)(nil)

type ErrInitConfigTimeout time.Duration

func (e ErrInitConfigTimeout) Error() string {
	return fmt.Sprintf("control init config exceed %s", time.Duration(e))
}

func (s *grpcServer) IPv6(_ context.Context, in *internal.Null) (*internal.Bool, error) {
	return &internal.Bool{Val: s.hdr.IPv6()}, nil
}

func (s *grpcServer) EndConfig(_ context.Context, in *internal.Null) (*internal.Null, error) {
	return &internal.Null{}, nil
}

func (s *grpcServer) AddTCP(_ context.Context, in *internal.String) (*internal.Session, error) {
	addr, err := netip.ParseAddrPort(in.Str)
	if err != nil {
		return &internal.Session{Err: internal.Eg(err)}, err
	}
	id, err := s.hdr.AddTCP(addr)
	if err != nil {
		return &internal.Session{Err: internal.Eg(err)}, err
	}
	return &internal.Session{ID: uint32(id)}, nil
}
func (s *grpcServer) AddUDP(_ context.Context, in *internal.String) (*internal.Session, error) {
	addr, err := netip.ParseAddrPort(in.Str)
	if err != nil {
		return &internal.Session{Err: internal.Eg(err)}, err
	}
	id, err := s.hdr.AddUDP(addr)
	if err != nil {
		return &internal.Session{Err: internal.Eg(err)}, err
	}
	return &internal.Session{ID: uint32(id)}, nil
}
func (s *grpcServer) DelTCP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelTCP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *grpcServer) DelUDP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelUDP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *grpcServer) PackLoss(_ context.Context, in *internal.Null) (*internal.Float32, error) {
	return &internal.Float32{Val: s.hdr.PackLoss()}, nil
}
func (s *grpcServer) Ping(_ context.Context, in *internal.Null) (*internal.Null, error) {
	return &internal.Null{}, nil
}

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
	ctx    context.Context
	cancel context.CancelFunc

	accetped atomic.Bool
	conn     net.Conn
}

var _ net.Listener = (*listenerWrap)(nil)

// newListenerWrap wrap the conn to a listener
func newListenerWrap(ctx context.Context, conn net.Conn) *listenerWrap {
	var l = &listenerWrap{conn: conn}
	l.ctx, l.cancel = context.WithCancel(ctx)
	return l
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

func (l *listenerWrap) Close() error {
	l.cancel()
	return nil
}

func (l *listenerWrap) Addr() net.Addr {
	return l.conn.LocalAddr()
}

type connWrap struct {
	net.Conn
	closed  chan struct{}
	closeMu sync.RWMutex
}

func wrapConn(conn net.Conn) *connWrap {
	return &connWrap{
		Conn:   conn,
		closed: make(chan struct{}),
	}
}

func (c *connWrap) Close() (err error) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	select {
	case <-c.closed:
	default:
		err = c.Conn.Close()
		close(c.closed)
	}
	return err
}

func (c *connWrap) Done() <-chan struct{} {
	return c.closed
}
