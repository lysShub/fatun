package control

import (
	"context"
	"encoding/gob"
	"net"
	"net/netip"

	"github.com/lysShub/itun/control/internal"
	"github.com/lysShub/itun/session"
	pkge "github.com/pkg/errors"
)

type Handler interface {
	IPv6() bool
	EndConfig()
	AddTCP(addr netip.AddrPort) (session.ID, error)
	DelTCP(id session.ID) error
	AddUDP(addr netip.AddrPort) (session.ID, error)
	DelUDP(id session.ID) error
	PackLoss() float32
	Ping()
}

type gobServer struct {
	conn net.Conn
	hdr  Handler

	enc *gob.Encoder
	dec *gob.Decoder
}

func newGobServer(tcp net.Conn, hdr Handler) *gobServer {
	var s = &gobServer{
		hdr:  hdr,
		conn: tcp,
		enc:  gob.NewEncoder(tcp),
		dec:  gob.NewDecoder(tcp),
	}
	return s
}

func (s *gobServer) Serve(ctx context.Context) error {
	defer s.conn.Close()

	var t internal.CtrType
	var err error
	for {
		t, err = s.nextType()
		if err != nil {
		} else if err = t.Valid(); err != nil {
		} else {
			switch t {
			case internal.IPv6:
				err = s.handleIPv6()
			case internal.EndConfig:
				err = s.handleEndConfig()
			case internal.AddTCP:
				err = s.handleAddTCP()
			case internal.DelTCP:
				err = s.handleDelTCP()
			case internal.AddUDP:
				err = s.handleAddUDP()
			case internal.DelUDP:
				err = s.handleDelUDP()
			case internal.PackLoss:
				err = s.handlePackLoss()
			case internal.Ping:
				err = s.handlePing()
			default:
				err = pkge.Errorf("not support control type %d", int(t))
			}
		}

		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
	}
}

func (s *gobServer) nextType() (t internal.CtrType, err error) {
	return t, s.dec.Decode(&t)
}

func (s *gobServer) handleIPv6() error {
	var req internal.IPv6Req
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	ipv6 := s.hdr.IPv6()

	var resp internal.IPv6Resp = internal.IPv6Resp(ipv6)
	return s.enc.Encode(resp)
}

func (s *gobServer) handleEndConfig() error {
	var req internal.EndConfigReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	s.hdr.EndConfig()

	var resp internal.EndConfigResp
	return s.enc.Encode(resp)
}

func (s *gobServer) handleAddTCP() error {
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

func (s *gobServer) handleDelTCP() error {
	var req internal.DelTCPReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	_ = s.hdr.DelTCP(req)

	var resp internal.DelTCPResp
	return s.enc.Encode(resp)
}

func (s *gobServer) handleAddUDP() error {
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

func (s *gobServer) handleDelUDP() error {
	var req internal.DelUDPReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	_ = s.hdr.DelUDP(req)

	var resp internal.DelUDPResp
	return s.enc.Encode(resp)
}

func (s *gobServer) handlePackLoss() error {
	var req internal.PackLossReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	pl := s.hdr.PackLoss()

	var resp internal.PackLossResp = internal.PackLossResp(pl)
	return s.enc.Encode(resp)
}

func (s *gobServer) handlePing() error {
	var req internal.PingReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	s.hdr.Ping()

	var resp internal.PingResp
	return s.enc.Encode(resp)
}
