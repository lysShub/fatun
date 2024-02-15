package internal

import (
	"bytes"
	"encoding/gob"
)

func Encode[T MgrTypes](src *T) (dst []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, 36))
	err = gob.NewEncoder(buf).Encode(src)
	return buf.Bytes(), err
}

func Decode[T MgrTypes](src []byte, dst *T) error {
	return gob.NewDecoder(bytes.NewReader(src)).Decode(dst)
}

type MgrTypes interface {
	CryptoReq | CryptoResp | IPv6Req | IPv6Resp | EndConfigReq | EndConfigResp | AddTCPReq | AddTCPResp | DelTCPReq | DelTCPResp | AddUDPReq | AddUDPResp | DelUDPReq | DelUDPResp | PackLossReq | PackLossResp | PingReq | PingResp
}

type CryptoReq struct{ Crypto bool }
type CryptoResp struct{}

type IPv6Req struct{}
type IPv6Resp struct{ IPv6 bool }

type EndConfigReq struct{}
type EndConfigResp struct{}

type AddTCPReq struct{ Addr string }
type AddTCPResp struct{ SessionID uint32 }

type DelTCPReq struct{ Addr string }
type DelTCPResp struct{ Error string }

type AddUDPReq struct{ Addr string }
type AddUDPResp struct{ SessionID uint32 }

type DelUDPReq struct{ Addr string }
type DelUDPResp struct{ Error string }

type PackLossReq struct{}
type PackLossResp struct{ PL float32 }

type PingReq struct{}
type PingResp struct{}
