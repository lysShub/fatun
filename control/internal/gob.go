package internal

import (
	"github.com/lysShub/itun/session"
	"github.com/pkg/errors"
)

//go:generate stringer -output gob_gen.go -trimprefix=CtrType -type=CtrType
type CtrType uint16

func (c CtrType) Valid() error {
	if c <= start || c >= end {
		return errors.Errorf("invalid control type %s", c)
	}
	return nil
}

const (
	start CtrType = iota

	IPv6
	EndConfig
	AddSession
	DelSession
	PackLoss
	Ping

	end
)

type IPv6Req struct{}
type IPv6Resp bool

type EndConfigReq struct{}
type EndConfigResp struct{}

type AddSessionReq = session.Session
type AddSessionResp struct {
	ID  session.ID
	Err error
}

type DelSessionReq = session.ID
type DelSessionResp struct{}

type PackLossReq struct{}
type PackLossResp float32

type PingReq struct{}
type PingResp struct{}
