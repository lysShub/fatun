package control

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control/internal"
	"github.com/lysShub/itun/ustack"
)

type SrvHandler interface {
	IPv6() bool
	EndConfig()
	AddTCP(addr netip.AddrPort) (uint16, error)
	DelTCP(id uint16) error
	AddUDP(addr netip.AddrPort) (uint16, error)
	DelUDP(id uint16) error
	PackLoss() float32
	Ping()
}

func Serve(ctx cctx.CancelCtx, ctr *Controller, hander SrvHandler) {
	tcp := ctr.stack.Accept(ctx, ctr.handshakeTimeout)
	if ctx.Err() != nil {
		return
	}

	srv := newGobServer(tcp, hander)
	srv.Serve(ctx)
}

type gobServer struct {
	hdr SrvHandler

	conn net.Conn

	enc *gob.Encoder
	dec *gob.Decoder
}

func newGobServer(tcp net.Conn, hdr SrvHandler) *gobServer {
	var s = &gobServer{
		hdr:  hdr,
		conn: tcp,
		enc:  gob.NewEncoder(tcp),
		dec:  gob.NewDecoder(tcp),
	}
	return s
}

func (s *gobServer) Serve(ctx cctx.CancelCtx) {
	var t internal.CtrType
	var err error
	for {
		// todo: s.conn support ctx
		select {
		case <-ctx.Done():
			return
		default:
		}

		t, err = s.nextType()
		if err != nil {
		} else if err = t.Valid(); err != nil {
		} else {

			switch t {
			case internal.IPv6:
				err = s.IPv6()
			case internal.EndConfig:
				err = s.EndConfig()
			case internal.AddTCP:
				err = s.AddTCP()
			case internal.DelTCP:
				err = s.DelTCP()
			case internal.AddUDP:
				err = s.AddUDP()
			case internal.DelUDP:
				err = s.DelUDP()
			case internal.PackLoss:
				err = s.PackLoss()
			case internal.Ping:
				err = s.Ping()
			default:
				err = fmt.Errorf("not support control type %d", int(t))
			}
		}

		if err != nil {
			err = errors.Join(
				err,
				s.conn.Close(),
			)

			err = errors.Join(
				err,
				ustack.WaitTCPClose(s.conn),
			)

			ctx.Cancel(err)
			return
		}
	}
}

func (s *gobServer) nextType() (t internal.CtrType, err error) {
	return t, s.dec.Decode(&t)
}

func (s *gobServer) IPv6() error {
	var req internal.IPv6Req
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	ipv6 := s.hdr.IPv6()

	var resp internal.IPv6Resp = internal.IPv6Resp(ipv6)
	return s.enc.Encode(resp)
}

func (s *gobServer) EndConfig() error {
	var req internal.EndConfigReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	s.hdr.EndConfig()

	var resp internal.EndConfigResp
	return s.enc.Encode(resp)
}

func (s *gobServer) AddTCP() error {
	var req internal.AddTCPReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	id, err := s.hdr.AddTCP(netip.AddrPort(req))

	var resp internal.AddTCPResp = internal.AddTCPResp{
		ID:  id,
		Err: err,
	}
	return s.enc.Encode(resp)
}

func (s *gobServer) DelTCP() error {
	var req internal.DelTCPReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	_ = s.hdr.DelTCP(uint16(req))

	var resp internal.DelTCPResp
	return s.enc.Encode(resp)
}

func (s *gobServer) AddUDP() error {
	var req internal.AddUDPReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	id, err := s.hdr.AddUDP(netip.AddrPort(req))

	var resp internal.AddUDPResp = internal.AddUDPResp{
		ID:  id,
		Err: err,
	}
	return s.enc.Encode(resp)
}

func (s *gobServer) DelUDP() error {
	var req internal.DelUDPReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	_ = s.hdr.DelUDP(uint16(req))

	var resp internal.DelUDPResp
	return s.enc.Encode(resp)
}

func (s *gobServer) PackLoss() error {
	var req internal.PackLossReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	pl := s.hdr.PackLoss()

	var resp internal.PackLossResp = internal.PackLossResp(pl)
	return s.enc.Encode(resp)
}

func (s *gobServer) Ping() error {
	var req internal.PingReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	s.hdr.Ping()

	var resp internal.PingResp
	return s.enc.Encode(resp)
}
