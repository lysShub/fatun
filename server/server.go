package server

import (
	"context"
	"net"
)

type Server struct {
	listener net.Listener
	portMgr  *PortMgr

	ctx context.Context
}

func NewServer(proxyConn net.Conn) *Server {
	return nil
}

func (s *Server) Server() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			panic(err)
		}

		go (&proxy{
			clientConn: conn,
		}).proxy()
	}
}
