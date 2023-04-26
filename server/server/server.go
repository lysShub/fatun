package server

import (
	"itun/server"
	"itun/server/ports"
	"itun/server/raw"
	"net"
	"net/netip"
	"sync"
)

type Handler interface {
	Proxy(s server.Server, conn net.Conn)
}

type ServerMux struct {
	raw.RawConn

	ports.PortMgr

	listener net.Listener
	locIP    netip.Addr

	handle Handler

	m *sync.RWMutex
}

var DefaultMux = &ServerMux{m: &sync.RWMutex{}, handle: defaultHandle}

func ListenAndServe(listener net.Listener) error {
	var locIP netip.Addr

	DefaultMux.listener = listener
	DefaultMux.locIP = locIP

	var err error
	DefaultMux.RawConn, err = raw.NewRawConn(locIP)
	if err != nil {
		return err
	}
	DefaultMux.PortMgr = ports.NewPorts(locIP)
	if err != nil {
		return err
	}

	return DefaultMux.do()
}

func (s *ServerMux) do() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		} else {
			s.handle.Proxy(s, conn)
		}
	}
}
