package control

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control/internal"
	"google.golang.org/grpc"
)

type CtrServer interface {
	IPv6() bool
	EndConfig()
	AddTCP(addr netip.AddrPort) (uint16, error)
	DelTCP(id uint16) error
	AddUDP(addr netip.AddrPort) (uint16, error)
	DelUDP(id uint16) error
	PackLoss() float32
	Ping()
}

type Server struct {
	internal.UnimplementedControlServer

	ctx      cctx.CancelCtx
	listener net.Listener

	srv *grpc.Server

	hdr CtrServer

	initedConfig atomic.Bool
}

func serve(ctx cctx.CancelCtx, initCfgTimeout time.Duration, conn net.Conn, hdr CtrServer) {
	var s = &Server{
		ctx:      ctx,
		listener: newListenerWrap(ctx, conn),
		srv:      grpc.NewServer(),
		hdr:      hdr,
	}
	internal.RegisterControlServer(s.srv, s)

	// todo: maybe not need, has keepalive
	go func() {
		time.Sleep(initCfgTimeout)
		if !s.initedConfig.Load() {
			s.ctx.Cancel(ErrInitConfigTimeout(initCfgTimeout))
		}
	}()

	// serve
	err := s.srv.Serve(s.listener)
	if err != nil {
		s.ctx.Cancel(err)
	}
}

type ErrInitConfigTimeout time.Duration

func (e ErrInitConfigTimeout) Error() string {
	return fmt.Sprintf("control init config exceed %s", time.Duration(e))
}

func (s *Server) IPv6(_ context.Context, in *internal.Null) (*internal.Bool, error) {
	return &internal.Bool{Val: s.hdr.IPv6()}, nil
}
func (s *Server) EndConfig(_ context.Context, in *internal.Null) (*internal.Null, error) {
	s.initedConfig.CompareAndSwap(false, true)
	return &internal.Null{}, nil
}

func (s *Server) AddTCP(_ context.Context, in *internal.String) (*internal.Session, error) {
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
func (s *Server) AddUDP(_ context.Context, in *internal.String) (*internal.Session, error) {
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
func (s *Server) DelTCP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelTCP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *Server) DelUDP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelUDP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *Server) PackLoss(_ context.Context, in *internal.Null) (*internal.Float32, error) {
	return &internal.Float32{Val: s.hdr.PackLoss()}, nil
}
func (s *Server) Ping(_ context.Context, in *internal.Null) (*internal.Null, error) {
	return &internal.Null{}, nil
}
