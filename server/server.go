package server

import (
	"context"
	"itun/server/ports"
	"itun/server/raw"
	"net"
	"net/netip"
	"sync"
)

type serverMux struct {
	context.Context
	raw.RawConn

	ports.PortMgr

	listener net.Listener
	localIP  netip.Addr

	m *sync.RWMutex
}

func ListenAndServe(listener net.Listener) error {
	var locIP netip.Addr

	var mux = &serverMux{m: &sync.RWMutex{}}
	mux.listener = listener
	mux.localIP = locIP

	var err error
	mux.RawConn, err = raw.NewRawConn(locIP)
	if err != nil {
		return err
	}
	mux.PortMgr = ports.NewPorts(locIP)
	if err != nil {
		return err
	}

	return mux.do()
}

func (s *serverMux) do() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		} else {
			Connect(s, conn)
		}
	}
}
