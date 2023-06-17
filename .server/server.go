package proxy

import (
	"context"
	"itun/pack"
	"itun/proxy/ports"
	"itun/proxy/raw"
	"net"
	"net/netip"
	"sync"
)

type serverMux struct {
	context.Context
	raw.RawConn
	pack.Pack

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

	return mux.handle()
}

func (s *serverMux) handle() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		} else {
			Connect(s, conn)
		}
	}
}
