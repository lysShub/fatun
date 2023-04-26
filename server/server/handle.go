package server

import (
	"itun/server"
	"itun/server/user"
	"net"
)

type Proxy struct {
}

var _ Handler = &Proxy{}

var defaultHandle = &Proxy{}

func (p *Proxy) Proxy(s server.Server, conn net.Conn) {
	user.NewUser(s, conn).Do()
}
