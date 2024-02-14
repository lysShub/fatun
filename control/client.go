package control

import "net"

func NewCtrClient(conn net.Conn) *Client {
	return &Client{}
}

type Client struct {
}
