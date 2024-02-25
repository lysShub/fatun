package control

import (
	"context"
	"encoding/gob"
	"errors"
	"net"
	"net/netip"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control/internal"
	"github.com/lysShub/itun/ustack"
)

type Client interface {
	Close() error

	IPv6(ctx context.Context) (bool, error)
	EndConfig(ctx context.Context) error
	AddTCP(ctx context.Context, addr netip.AddrPort) (uint16, error)
	DelTCP(ctx context.Context, id uint16) error
	AddUDP(ctx context.Context, addr netip.AddrPort) (uint16, error)
	DelUDP(ctx context.Context, id uint16) error
	PackLoss(ctx context.Context) (float32, error)
	Ping(ctx context.Context) error
}

type Client2 interface {
	Close() error

	IPv6() (bool, error)
	EndConfig() error
	AddTCP(addr netip.AddrPort) (*AddTCP, error)
	DelTCP(id uint16) error
	AddUDP(addr netip.AddrPort) (*AddUDP, error)
	DelUDP(id uint16) error
	PackLoss() (float32, error)
	Ping() error
}

func Dial(ctx cctx.CancelCtx, ctr *Controller) Client2 {
	tcp := ctr.stack.Connect(ctx, ctr.handshakeTimeout)
	if ctx.Err() != nil {
		return nil
	}

	// return newGrpcClient(ctx, ctr, tcp, ctr.handshakeTimeout*3) // todo: from cfg

	return newGobClient(ctx, tcp)

}

/*














 */

type gobClient struct {
	ctx cctx.CancelCtx

	conn net.Conn

	enc *gob.Encoder
	dec *gob.Decoder
}

func newGobClient(parentCtx cctx.CancelCtx, tcp net.Conn) *gobClient {
	return &gobClient{
		ctx:  parentCtx,
		conn: tcp,
		enc:  gob.NewEncoder(tcp),
		dec:  gob.NewDecoder(tcp),
	}
}

var _ Client2 = (*gobClient)(nil)

func (c *gobClient) Close() error {
	err := c.conn.Close()

	err = errors.Join(
		err,
		ustack.WaitTCPClose(c.conn),
	)

	c.ctx.Cancel(nil)
	return err
}

func (c *gobClient) nextType(t internal.CtrType) error {
	return c.enc.Encode(t)
}

func (c *gobClient) IPv6() (bool, error) {
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

func (c *gobClient) EndConfig() error {
	if err := c.nextType(internal.EndConfig); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.EndConfigReq{}); err != nil {
		return err
	}
	return c.dec.Decode(&internal.EndConfigResp{})
}

type AddTCP internal.AddTCPResp

func (c *gobClient) AddTCP(addr netip.AddrPort) (*AddTCP, error) {
	if err := c.nextType(internal.AddTCP); err != nil {
		return nil, err
	}
	if err := c.enc.Encode(internal.AddTCPReq(addr)); err != nil {
		return nil, err
	}
	var resp AddTCP
	err := c.dec.Decode(&resp)
	return &resp, err
}

func (c *gobClient) DelTCP(id uint16) error {
	if err := c.nextType(internal.DelTCP); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.DelTCPReq(id)); err != nil {
		return err
	}
	var resp internal.DelTCPResp
	err := c.dec.Decode(&resp)
	return err
}

type AddUDP internal.AddUDPResp

func (c *gobClient) AddUDP(addr netip.AddrPort) (*AddUDP, error) {
	if err := c.nextType(internal.AddUDP); err != nil {
		return nil, err
	}
	if err := c.enc.Encode(internal.AddUDPReq(addr)); err != nil {
		return nil, err
	}
	var resp AddUDP
	err := c.dec.Decode(&resp)
	return &resp, err
}

func (c *gobClient) DelUDP(id uint16) error {
	if err := c.nextType(internal.DelUDP); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.DelUDPReq(id)); err != nil {
		return err
	}
	var resp internal.DelUDPResp
	err := c.dec.Decode(&resp)
	return err
}

func (c *gobClient) PackLoss() (float32, error) {
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

func (c *gobClient) Ping() error {
	if err := c.nextType(internal.Ping); err != nil {
		return err
	}
	if err := c.enc.Encode(internal.PingReq{}); err != nil {
		return err
	}
	return c.dec.Decode(&internal.PingResp{})
}
