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

type MgrHander interface {
	IPv6() bool
	EndConfig()
	AddTCP(addr netip.AddrPort) (uint16, error)
	DelTCP(id uint16) error
	AddUDP(addr netip.AddrPort) (uint16, error)
	DelUDP(id uint16) error
	PackLoss() float32
	Ping()
}

func Serve(ctx cctx.CancelCtx, conn net.Conn, hdr MgrHander, initCfgTimeout time.Duration) {
	s := newServer(hdr)
	s.serve(ctx, conn, initCfgTimeout)
}

func newServer(hdr MgrHander) *server {
	var s = &server{
		srv: grpc.NewServer(),
		hdr: hdr,
	}
	internal.RegisterControlServer(s.srv, s)
	return s
}

type server struct {
	internal.UnimplementedControlServer

	srv *grpc.Server

	hdr MgrHander

	initedConfig atomic.Bool
}

type ErrInitConfigTimeout time.Duration

func (e ErrInitConfigTimeout) Error() string {
	return fmt.Sprintf("control init config exceed %s", time.Duration(e))
}

func (s *server) serve(ctx cctx.CancelCtx, conn net.Conn, initCfgTimeout time.Duration) {
	go func() {
		time.Sleep(initCfgTimeout)
		if !s.initedConfig.Load() {
			ctx.Cancel(ErrInitConfigTimeout(initCfgTimeout))
		}
	}()

	err := s.srv.Serve(newListenerWrap(ctx, conn))
	if err != nil {
		ctx.Cancel(err)
	}
}

func (s *server) IPv6(_ context.Context, in *internal.Null) (*internal.Bool, error) {
	return &internal.Bool{Val: s.hdr.IPv6()}, nil
}
func (s *server) EndConfig(_ context.Context, in *internal.Null) (*internal.Null, error) {
	s.initedConfig.CompareAndSwap(false, true)
	return &internal.Null{}, nil
}

func (s *server) AddTCP(_ context.Context, in *internal.String) (*internal.Session, error) {
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
func (s *server) AddUDP(_ context.Context, in *internal.String) (*internal.Session, error) {
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
func (s *server) DelTCP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelTCP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *server) DelUDP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelUDP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *server) PackLoss(_ context.Context, in *internal.Null) (*internal.Float32, error) {
	return &internal.Float32{Val: s.hdr.PackLoss()}, nil
}
func (s *server) Ping(_ context.Context, in *internal.Null) (*internal.Null, error) {
	return &internal.Null{}, nil
}
