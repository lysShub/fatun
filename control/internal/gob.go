package internal

import (
	"net/netip"

	"github.com/lysShub/itun/session"
	pkge "github.com/pkg/errors"
)

//go:generate stringer -output gob_gen.go -trimprefix=CtrType -type=CtrType
type CtrType uint16

func (c CtrType) Valid() error {
	if c <= start || c >= end {
		return pkge.Errorf("invalid control type %s", c)
	}
	return nil
}

const (
	start CtrType = iota

	IPv6
	EndConfig
	AddTCP
	DelTCP
	AddUDP
	DelUDP
	PackLoss
	Ping

	end
)

type IPv6Req struct{}
type IPv6Resp bool

type EndConfigReq struct{}
type EndConfigResp struct{}

type AddTCPReq = netip.AddrPort
type AddTCPResp struct {
	ID  session.ID
	Err error
}

type DelTCPReq = session.ID
type DelTCPResp struct{}

type AddUDPReq netip.AddrPort
type AddUDPResp struct {
	ID  session.ID
	Err error
}

type DelUDPReq = session.ID
type DelUDPResp struct{}

type PackLossReq struct{}
type PackLossResp float32

type PingReq struct{}
type PingResp struct{}
