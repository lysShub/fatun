//go:build linux
// +build linux

package proxyer

import (
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/session"
)

type proxyerImpl Proxyer

type proxyerImplPtr = *proxyerImpl

var _ control.Handler = (proxyerImplPtr)(nil)

func (pi *proxyerImpl) IPv6() bool {
	return true
}

func (pi *proxyerImpl) EndConfig() {
	select {
	case <-pi.endConfigNotify:
	default:
		close(pi.endConfigNotify)
	}
}

// todo: error 不要把server堆栈返回处理咯
func (pi *proxyerImpl) AddTCP(addr netip.AddrPort) (session.ID, error) {
	s, err := pi.sessionMgr.Add(
		pi.ctx, (*Proxyer)(pi),
		session.Session{
			SrcAddr: pi.raw.RemoteAddrPort(),
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
	return pi.sessionMgr.Del(id, nil)
}

func (pi *proxyerImpl) AddUDP(addr netip.AddrPort) (session.ID, error) {
	s, err := pi.sessionMgr.Add(
		pi.ctx, (*Proxyer)(pi),
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
	return pi.sessionMgr.Del(id, nil)
}

func (pi *proxyerImpl) PackLoss() float32 {
	return 0
}

func (pi *proxyerImpl) Ping() {
}
