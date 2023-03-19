package server

import (
	"context"
	"net"
)

type Server struct {
	listener net.Listener

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

		go (&user{
			clientConn: conn,
		}).proxy()
	}
}
