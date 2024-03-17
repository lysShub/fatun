package control

import (
	"context"
	"encoding/gob"
	"net"
	"time"

	"github.com/lysShub/itun/control/internal"
	"github.com/pkg/errors"
)

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

func (s *gobServer) Close() error {
	return s.conn.Close()
}

func (s *gobServer) Serve(ctx context.Context) error {
	var retCh = make(chan struct{})
	defer close(retCh)
	go func() {
		select {
		case <-ctx.Done():
			s.conn.SetDeadline(time.Now())
		case <-retCh:
		}
	}()

	var t internal.CtrType
	var err error
	for {
		t, err = s.nextType()
		if err != nil {
			return errors.WithStack(err)
		} else if err = t.Valid(); err != nil {
			return err
		} else {
			switch t {
			case internal.IPv6:
				err = s.handleIPv6()
			case internal.EndConfig:
				err = s.handleEndConfig()
			case internal.AddSession:
				err = s.handleAddSession()
			case internal.DelSession:
				err = s.handleDelSession()
			case internal.PackLoss:
				err = s.handlePackLoss()
			case internal.Ping:
				err = s.handlePing()
			default:
				err = errors.Errorf("not support control type %d", int(t))
			}

			if err != nil {
				return errors.WithStack(err)
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

func (s *gobServer) handleAddSession() error {
	var req internal.AddSessionReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	id, err := s.hdr.AddSession(req)

	var resp = internal.AddSessionResp{
		ID:  id,
		Err: err,
	}
	return s.enc.Encode(resp)
}

func (s *gobServer) handleDelSession() error {
	var req internal.DelSessionReq
	if err := s.dec.Decode(&req); err != nil {
		return err
	}

	_ = s.hdr.DelSession(req)

	var resp internal.DelSessionResp
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
