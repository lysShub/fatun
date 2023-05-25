package proxy

import (
	"context"
	"itun/proxy/handle"
	"itun/proxy/rule"
	"net"
)

type Proxy struct {
	proxyConn net.Conn

	Rules
}

func ListenAndProxy(ctx context.Context, pxyConn net.Conn, cfg *Config) *Proxy {
	if cfg == nil {
		cfg = &Config{Ipv6: true}
	}

	var p = &Proxy{
		proxyConn: pxyConn,
		Rules:     rule.NewRules(),
	}

	go func() {
		pch := p.Proxyer()
		for f := range pch {
			handle.Handle(p.proxyConn, f)
		}
	}()
	return p
}

type Config struct {
	IfIdxs []int
	Ipv6   bool // support ipv6
}

type Rules interface {
	Proxyer() <-chan string
	AddRule(rule string) error
	AddBuiltinRule() error
	DelRule(rule string) error
	List() []string
}
