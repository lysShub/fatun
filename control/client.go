package control

import (
	"context"
	"encoding/gob"
	"net"

	"github.com/lysShub/itun/control/internal"
	"github.com/lysShub/itun/session"
)

// todo: support ctx, e.g: ctr.IPv6(ctx)

type gobClient struct {
	conn net.Conn

	enc *gob.Encoder
	dec *gob.Decoder
}

func newGobClient(tcp net.Conn) *gobClient {
	return &gobClient{
		conn: tcp,
		enc:  gob.NewEncoder(tcp),
		dec:  gob.NewDecoder(tcp),
	}
}

var _ Client = (*gobClient)(nil)

func (c *gobClient) Close() (err error) {
	return c.conn.Close()
}

func (c *gobClient) nextType(t internal.CtrType) error {
	return c.enc.Encode(t)
}

func (c *gobClient) IPv6(ctx context.Context) (bool, error) {
	if err := c.nextType(internal.IPv6); err != nil {
		return false, err
	}
	if err := c.enc.Encode(internal.IPv6Req{}); err != nil {
		return false, err
	}

	var resp internal.IPv6Resp
	err := c.dec.Decode(&resp)
	return bool(resp), err
}

func (c *gobClient) EndConfig(ctx context.Context) error {
	if err := c.nextType(internal.EndConfig); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.EndConfigReq{}); err != nil {
		return err
	}
	return c.dec.Decode(&internal.EndConfigResp{})
}

type AddSession internal.AddSessionResp

func (c *gobClient) AddSession(ctx context.Context, s session.Session) (*AddSession, error) {
	if err := c.nextType(internal.AddSession); err != nil {
		return nil, err
	}
	if err := c.enc.Encode(internal.AddSessionReq(s)); err != nil {
		return nil, err
	}
	var resp = &internal.AddSessionResp{}
	err := c.dec.Decode(resp)
	return (*AddSession)(resp), err
}

func (c *gobClient) DelSession(ctx context.Context, id session.ID) error {
	if err := c.nextType(internal.DelSession); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.DelSessionReq(id)); err != nil {
		return err
	}
	var resp internal.DelSessionResp
	err := c.dec.Decode(&resp)
	return err
}

func (c *gobClient) PackLoss(ctx context.Context) (float32, error) {
	if err := c.nextType(internal.PackLoss); err != nil {
		return 0, err
	}
	if err := c.enc.Encode(internal.PackLossReq{}); err != nil {
		return 0, err
	}
	var resp internal.PackLossResp
	err := c.dec.Decode(&resp)
	return float32(resp), err
}

func (c *gobClient) Ping(ctx context.Context) error {
	if err := c.nextType(internal.Ping); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.PingReq{}); err != nil {
		return err
	}
	return c.dec.Decode(&internal.PingResp{})
}
