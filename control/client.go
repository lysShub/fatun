package control

import (
	"context"
	"errors"
	"net"
	"net/netip"

	"github.com/lysShub/itun/control/internal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewCtrClient(conn net.Conn) (*Client, error) {

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {

			return conn, nil
		}),
	}

	gconn, err := grpc.Dial("", opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: internal.NewControlClient(gconn),
	}, nil
}

type Client struct {
	client internal.ControlClient
}

var ctx = context.Background()

func (c *Client) Crypto(crypto bool) error {
	_, err := c.client.Crypto(ctx, &internal.Bool{Val: crypto})
	return err
}
func (c *Client) IPv6() (bool, error) {
	r, err := c.client.IPv6(ctx, &internal.Null{})
	return r.Val, err
}
func (c *Client) EndConfig() error {
	_, err := c.client.EndConfig(ctx, &internal.Null{})
	return err
}
func (c *Client) AddTCP(addr netip.AddrPort) (uint16, error) {
	s, err := c.client.AddTCP(ctx, &internal.String{Str: addr.String()})
	return uint16(s.ID), errors.Join(err, internal.Ge(s.Err))
}
func (c *Client) DelTCP(id uint16) error {
	s, err := c.client.DelTCP(ctx, &internal.SessionID{ID: uint32(id)})
	return errors.Join(err, internal.Ge(s))
}
func (c *Client) AddUDP(addr netip.AddrPort) (uint16, error) {
	s, err := c.client.AddUDP(ctx, &internal.String{Str: addr.String()})
	return uint16(s.ID), errors.Join(err, internal.Ge(s.Err))
}
func (c *Client) DelUDP(id uint16) error {
	s, err := c.client.DelUDP(ctx, &internal.SessionID{ID: uint32(id)})
	return errors.Join(err, internal.Ge(s))
}
func (c *Client) PackLoss() (float32, error) {
	r, err := c.client.PackLoss(ctx, &internal.Null{})
	return r.Val, err
}
func (c *Client) Ping() error {
	_, err := c.client.Ping(ctx, &internal.Null{})
	return err
}
