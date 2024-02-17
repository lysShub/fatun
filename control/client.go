package control

import (
	"context"
	"errors"
	"net"
	"net/netip"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/control/internal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewCtrClient(ctx cctx.CancelCtx, conn net.Conn) *Client {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return conn, nil
		}),
	}

	gconn, err := grpc.Dial("", opts...)
	if err != nil {
		ctx.Cancel(err)
		return nil
	}

	return &Client{
		ctx:    ctx,
		client: internal.NewControlClient(gconn),
	}
}

type Client struct {
	ctx    cctx.CancelCtx
	client internal.ControlClient
}

func (c *Client) cancel(err error) bool {
	if err != nil {
		c.ctx.Cancel(err)
		return true
	}
	return false
}

func (c *Client) IPv6() bool {
	r, err := c.client.IPv6(c.ctx, &internal.Null{})
	c.cancel(err)
	return r.Val
}
func (c *Client) EndConfig() {
	_, err := c.client.EndConfig(c.ctx, &internal.Null{})
	c.cancel(err)
}
func (c *Client) AddTCP(addr netip.AddrPort) (uint16, error) {
	s, err := c.client.AddTCP(c.ctx, &internal.String{Str: addr.String()})
	return uint16(s.ID), errors.Join(err, internal.Ge(s.Err))
}
func (c *Client) DelTCP(id uint16) error {
	s, err := c.client.DelTCP(c.ctx, &internal.SessionID{ID: uint32(id)})
	return errors.Join(err, internal.Ge(s))
}
func (c *Client) AddUDP(addr netip.AddrPort) (uint16, error) {
	s, err := c.client.AddUDP(c.ctx, &internal.String{Str: addr.String()})
	return uint16(s.ID), errors.Join(err, internal.Ge(s.Err))
}
func (c *Client) DelUDP(id uint16) error {
	s, err := c.client.DelUDP(c.ctx, &internal.SessionID{ID: uint32(id)})
	return errors.Join(err, internal.Ge(s))
}
func (c *Client) PackLoss() float32 {
	r, err := c.client.PackLoss(c.ctx, &internal.Null{})
	c.cancel(err)
	return r.Val
}
func (c *Client) Ping() {
	_, err := c.client.Ping(c.ctx, &internal.Null{})
	c.cancel(err)
}
