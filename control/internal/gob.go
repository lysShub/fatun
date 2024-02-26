package internal

import (
	"net/netip"

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

type AddTCPReq netip.AddrPort
type AddTCPResp struct {
	ID  uint16
	Err error
}

type DelTCPReq uint16
type DelTCPResp struct{}

type AddUDPReq netip.AddrPort
type AddUDPResp struct {
	ID  uint16
	Err error
}

type DelUDPReq uint16
type DelUDPResp struct{}

type PackLossReq struct{}
type PackLossResp float32

type PingReq struct{}
type PingResp struct{}

func (IPv6Req) Type() CtrType      { return IPv6 }
func (EndConfigReq) Type() CtrType { return EndConfig }
func (AddTCPReq) Type() CtrType    { return AddTCP }
func (DelTCPReq) Type() CtrType    { return DelTCP }
func (AddUDPReq) Type() CtrType    { return AddUDP }
func (DelUDPReq) Type() CtrType    { return DelUDP }
func (PackLossReq) Type() CtrType  { return PackLoss }
func (PingReq) Type() CtrType      { return Ping }
