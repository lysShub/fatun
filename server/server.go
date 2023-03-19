package server

import "net"

type Server struct {
	listener net.Listener
}

func (s *Server) Do() {

	conn, err := s.listener.Accept()
	if err != nil {
		panic(err)
	}

	go func() {
		(&user{conn}).do()
	}()
}
