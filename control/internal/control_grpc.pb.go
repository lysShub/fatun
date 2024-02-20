// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: control.proto

package internal

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Control_IPv6_FullMethodName      = "/internal.Control/IPv6"
	Control_EndConfig_FullMethodName = "/internal.Control/EndConfig"
	Control_AddTCP_FullMethodName    = "/internal.Control/AddTCP"
	Control_DelTCP_FullMethodName    = "/internal.Control/DelTCP"
	Control_AddUDP_FullMethodName    = "/internal.Control/AddUDP"
	Control_DelUDP_FullMethodName    = "/internal.Control/DelUDP"
	Control_PackLoss_FullMethodName  = "/internal.Control/PackLoss"
	Control_Ping_FullMethodName      = "/internal.Control/Ping"
)

// ControlClient is the client API for Control service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ControlClient interface {
	IPv6(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Bool, error)
	EndConfig(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Null, error)
	AddTCP(ctx context.Context, in *String, opts ...grpc.CallOption) (*Session, error)
	DelTCP(ctx context.Context, in *SessionID, opts ...grpc.CallOption) (*Err, error)
	AddUDP(ctx context.Context, in *String, opts ...grpc.CallOption) (*Session, error)
	DelUDP(ctx context.Context, in *SessionID, opts ...grpc.CallOption) (*Err, error)
	PackLoss(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Float32, error)
	Ping(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Null, error)
}

type controlClient struct {
	cc grpc.ClientConnInterface
}

func NewControlClient(cc grpc.ClientConnInterface) ControlClient {
	return &controlClient{cc}
}

func (c *controlClient) IPv6(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Bool, error) {
	out := new(Bool)
	err := c.cc.Invoke(ctx, Control_IPv6_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) EndConfig(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Null, error) {
	out := new(Null)
	err := c.cc.Invoke(ctx, Control_EndConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) AddTCP(ctx context.Context, in *String, opts ...grpc.CallOption) (*Session, error) {
	out := new(Session)
	err := c.cc.Invoke(ctx, Control_AddTCP_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) DelTCP(ctx context.Context, in *SessionID, opts ...grpc.CallOption) (*Err, error) {
	out := new(Err)
	err := c.cc.Invoke(ctx, Control_DelTCP_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) AddUDP(ctx context.Context, in *String, opts ...grpc.CallOption) (*Session, error) {
	out := new(Session)
	err := c.cc.Invoke(ctx, Control_AddUDP_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) DelUDP(ctx context.Context, in *SessionID, opts ...grpc.CallOption) (*Err, error) {
	out := new(Err)
	err := c.cc.Invoke(ctx, Control_DelUDP_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) PackLoss(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Float32, error) {
	out := new(Float32)
	err := c.cc.Invoke(ctx, Control_PackLoss_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controlClient) Ping(ctx context.Context, in *Null, opts ...grpc.CallOption) (*Null, error) {
	out := new(Null)
	err := c.cc.Invoke(ctx, Control_Ping_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ControlServer is the server API for Control service.
// All implementations must embed UnimplementedControlServer
// for forward compatibility
type ControlServer interface {
	IPv6(context.Context, *Null) (*Bool, error)
	EndConfig(context.Context, *Null) (*Null, error)
	AddTCP(context.Context, *String) (*Session, error)
	DelTCP(context.Context, *SessionID) (*Err, error)
	AddUDP(context.Context, *String) (*Session, error)
	DelUDP(context.Context, *SessionID) (*Err, error)
	PackLoss(context.Context, *Null) (*Float32, error)
	Ping(context.Context, *Null) (*Null, error)
	mustEmbedUnimplementedControlServer()
}

// UnimplementedControlServer must be embedded to have forward compatible implementations.
type UnimplementedControlServer struct {
}

func (UnimplementedControlServer) IPv6(context.Context, *Null) (*Bool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IPv6 not implemented")
}
func (UnimplementedControlServer) EndConfig(context.Context, *Null) (*Null, error) {
	return nil, status.Errorf(codes.Unimplemented, "method EndConfig not implemented")
}
func (UnimplementedControlServer) AddTCP(context.Context, *String) (*Session, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddTCP not implemented")
}
func (UnimplementedControlServer) DelTCP(context.Context, *SessionID) (*Err, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DelTCP not implemented")
}
func (UnimplementedControlServer) AddUDP(context.Context, *String) (*Session, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddUDP not implemented")
}
func (UnimplementedControlServer) DelUDP(context.Context, *SessionID) (*Err, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DelUDP not implemented")
}
func (UnimplementedControlServer) PackLoss(context.Context, *Null) (*Float32, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PackLoss not implemented")
}
func (UnimplementedControlServer) Ping(context.Context, *Null) (*Null, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedControlServer) mustEmbedUnimplementedControlServer() {}

// UnsafeControlServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ControlServer will
// result in compilation errors.
type UnsafeControlServer interface {
	mustEmbedUnimplementedControlServer()
}

func RegisterControlServer(s grpc.ServiceRegistrar, srv ControlServer) {
	s.RegisterService(&Control_ServiceDesc, srv)
}

func _Control_IPv6_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Null)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).IPv6(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_IPv6_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).IPv6(ctx, req.(*Null))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_EndConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Null)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).EndConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_EndConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).EndConfig(ctx, req.(*Null))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_AddTCP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(String)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).AddTCP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_AddTCP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).AddTCP(ctx, req.(*String))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_DelTCP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SessionID)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).DelTCP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_DelTCP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).DelTCP(ctx, req.(*SessionID))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_AddUDP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(String)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).AddUDP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_AddUDP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).AddUDP(ctx, req.(*String))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_DelUDP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SessionID)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).DelUDP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_DelUDP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).DelUDP(ctx, req.(*SessionID))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_PackLoss_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Null)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).PackLoss(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_PackLoss_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).PackLoss(ctx, req.(*Null))
	}
	return interceptor(ctx, in, info, handler)
}

func _Control_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Null)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Control_Ping_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServer).Ping(ctx, req.(*Null))
	}
	return interceptor(ctx, in, info, handler)
}

// Control_ServiceDesc is the grpc.ServiceDesc for Control service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Control_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "internal.Control",
	HandlerType: (*ControlServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "IPv6",
			Handler:    _Control_IPv6_Handler,
		},
		{
			MethodName: "EndConfig",
			Handler:    _Control_EndConfig_Handler,
		},
		{
			MethodName: "AddTCP",
			Handler:    _Control_AddTCP_Handler,
		},
		{
			MethodName: "DelTCP",
			Handler:    _Control_DelTCP_Handler,
		},
		{
			MethodName: "AddUDP",
			Handler:    _Control_AddUDP_Handler,
		},
		{
			MethodName: "DelUDP",
			Handler:    _Control_DelUDP_Handler,
		},
		{
			MethodName: "PackLoss",
			Handler:    _Control_PackLoss_Handler,
		},
		{
			MethodName: "Ping",
			Handler:    _Control_Ping_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "control.proto",
}
