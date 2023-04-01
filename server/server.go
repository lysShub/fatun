package server

import (
	"fmt"
	"net"
)

type Server struct {
	listener net.Listener
}

func (s *Server) Do() {
	conn, err := s.listener.Accept()
	if err != nil {
		panic(err)
	} else {
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) error {
	raddr, err := toNetIP(conn.RemoteAddr())
	// TODO: resolve IP conn port
	if err != nil {
		return err
	}

	fmt.Println(raddr)
	return nil
}
