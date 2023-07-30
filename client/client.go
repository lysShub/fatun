package client

import (
	"itun/client/handle"
	"itun/client/rule"
	"net"

	"go.uber.org/zap"
)

type Client struct {
	proxyConn net.Conn

	rule.Rules

	logger *zap.Logger
}

func NewClient(pxyConn net.Conn) *Client {

	var p = &Client{
		proxyConn: pxyConn,
		Rules:     rule.NewRules(),
	}

	go func() {
		pch := p.Capture()
		for f := range pch {
			handle.Handle(p.proxyConn, f)
		}
	}()
	return p
}
