package server

import (
	"fmt"
	"itun/pack"
	"net"
)

type Server struct {
	listener net.Listener
}

func NewServer(proxyConn net.Conn) *Server {
	return nil
}

func (s *Server) server() {

	var b = make([]byte, 1532)
	var n int
	var addr *net.UDPAddr
	var err error
	var dstIP [4]byte

	for {

		fmt.Println(addr, dstIP)

		if n, _, err = s.proxyConn.ReadFromUDP(b); err != nil {
			fmt.Println(err.Error())
			return
		} else {
			if n > 40+pack.W {

			}
		}
	}
}
