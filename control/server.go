package control

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/itun/control/internal"
	"google.golang.org/grpc"
)

type MgrHander interface {
	Crypto(crypto bool)
	IPv6() bool
	EndConfig() error
	AddTCP(addr netip.AddrPort) (uint16, error)
	DelTCP(id uint16) error
	AddUDP(addr netip.AddrPort) (uint16, error)
	DelUDP(id uint16) error
	PackLoss() float32
	Ping()
}

func Serve(ctx context.Context, conn net.Conn, hdr MgrHander) error {
	s := grpc.NewServer()
	defer s.Stop()

	internal.RegisterControlServer(s, &server{hdr: hdr})
	return s.Serve(newListenerWrap(ctx, conn))
}

type server struct {
	internal.UnimplementedControlServer
	hdr MgrHander
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
func (s *server) Crypto(_ context.Context, in *internal.Bool) (*internal.Null, error) {
	s.hdr.Crypto(in.Val)
	return &internal.Null{}, nil
}
func (s *server) DelTCP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelTCP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *server) DelUDP(_ context.Context, in *internal.SessionID) (*internal.Err, error) {
	err := s.hdr.DelUDP(uint16(in.ID))
	return internal.Eg(err), err
}
func (s *server) EndConfig(_ context.Context, in *internal.Null) (*internal.Null, error) {
	return &internal.Null{}, nil
}
func (s *server) IPv6(_ context.Context, in *internal.Null) (*internal.Bool, error) {
	return &internal.Bool{Val: s.hdr.IPv6()}, nil
}
func (s *server) PackLoss(_ context.Context, in *internal.Null) (*internal.Float32, error) {
	return &internal.Float32{Val: s.hdr.PackLoss()}, nil
}
func (s *server) Ping(_ context.Context, in *internal.Null) (*internal.Null, error) {
	return &internal.Null{}, nil
}
