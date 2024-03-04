//go:build linux
// +build linux

package server

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/session"
)

type proxyerImpl proxyer

type proxyerImplPtr = *proxyerImpl

var _ control.SrvHandler = (proxyerImplPtr)(nil)

func (pi *proxyerImpl) IPv6() bool {
	return true
}

func (pi *proxyerImpl) EndConfig() {
	fmt.Println("完成初始化配置")
}

func (pi *proxyerImpl) AddTCP(addr netip.AddrPort) (session.ID, error) {
	s, err := pi.sessionMgr.Add(
		pi.ctx,
		session.Session{
			SrcAddr: pi.SrcAddr,
			Proto:   itun.TCP,
			DstAddr: addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (pi *proxyerImpl) DelTCP(id session.ID) error {
	return pi.sessionMgr.Del(id)
}

func (pi *proxyerImpl) AddUDP(addr netip.AddrPort) (session.ID, error) {
	s, err := pi.sessionMgr.Add(
		pi.ctx,
		session.Session{
			Proto:   itun.UDP,
			DstAddr: addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (pi *proxyerImpl) DelUDP(id session.ID) error {
	return pi.sessionMgr.Del(id)
}

func (pi *proxyerImpl) PackLoss() float32 {
	return 0
}

func (pi *proxyerImpl) Ping() {
}
