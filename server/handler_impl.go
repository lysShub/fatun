package server

import (
	"net/netip"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/control"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type handlerImpl handler

type handlerImplPtr = *handlerImpl

var _ control.MgrHander = (handlerImplPtr)(nil)

func (h *handlerImpl) IPv6() bool {
	return false
}

func (hi *handlerImpl) EndConfig() {}

func (hi *handlerImpl) AddTCP(addr netip.AddrPort) (uint16, error) {
	s, err := hi.sessionMgr.Add(
		hi.ctx,
		itun.Session{
			Proto:  header.TCPProtocolNumber,
			Server: addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (hi *handlerImpl) DelTCP(id uint16) error {
	return hi.sessionMgr.Del(id)
}

func (hi *handlerImpl) AddUDP(addr netip.AddrPort) (uint16, error) {
	s, err := hi.sessionMgr.Add(
		hi.ctx,
		itun.Session{
			Proto:  header.UDPProtocolNumber,
			Server: addr,
		},
	)
	if err != nil {
		return 0, err
	}
	return s.ID(), nil
}
func (hi *handlerImpl) DelUDP(id uint16) error {
	return hi.sessionMgr.Del(id)
}

func (hi *handlerImpl) PackLoss() float32 {
	return 0
}

func (hi *handlerImpl) Ping() {
}
