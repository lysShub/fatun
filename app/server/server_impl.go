package server

import (
	"context"
	"net"
	"net/netip"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server/adapter"
	"github.com/lysShub/itun/ustack"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) Config() *app.Config     { return s.cfg }
func (s *proxyerImpl) Adapter() *adapter.Ports { return s.ap }
func (s *proxyerImpl) Endpoint(client netip.AddrPort) (ustack.Endpoint, error) {
	ep, err := ustack.NewEndpoint(s.stack, s.raw.Addr().Port(), client)
	return ep, err
}
func (s *proxyerImpl) Accept(ctx context.Context, client netip.AddrPort) (net.Conn, error) {
	return s.l.AcceptBy(ctx, client)
}
