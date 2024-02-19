//go:build linux
// +build linux

package server

import (
	"net/netip"

	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/protocol"
)

type proxyerImpl proxyer

type proxyerImplPtr = *proxyerImpl

var _ control.MgrHander = (proxyerImplPtr)(nil)

func (pi *proxyerImpl) IPv6() bool {
	return false
}

func (pi *proxyerImpl) EndConfig() {}

func (pi *proxyerImpl) AddTCP(addr netip.AddrPort) (uint16, error) {
	s, err := pi.sessionMgr.Add(
		pi.ctx,
		protocol.Session{
			SrcAddr: pi.SrcAddr,
			Proto:   protocol.TCP,
			DstAddr: addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (pi *proxyerImpl) DelTCP(id uint16) error {
	return pi.sessionMgr.Del(id)
}

func (pi *proxyerImpl) AddUDP(addr netip.AddrPort) (uint16, error) {
	s, err := pi.sessionMgr.Add(
		pi.ctx,
		protocol.Session{
			Proto:   protocol.UDP,
			DstAddr: addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (pi *proxyerImpl) DelUDP(id uint16) error {
	return pi.sessionMgr.Del(id)
}

func (pi *proxyerImpl) PackLoss() float32 {
	return 0
}

func (pi *proxyerImpl) Ping() {
}
