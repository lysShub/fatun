package main

import (
	"io"
	"net"

	"github.com/boltdb/bolt"
	kcp "github.com/xtaci/kcp-go/v5"
)

type itunc struct {
	db *bolt.DB
	s  *net.TCPAddr
}

var pb []byte = []byte("pb")

func (i itunc) run(laddr *net.TCPAddr) {
	l, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		panic(err.Error())
	}
	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			panic(err.Error())
		}

		go i.do(conn)
	}
}

func (i itunc) do(iconn *net.TCPConn) {

	var r []byte

	if err := i.db.View(func(tx *bolt.Tx) error {
		_, r = tx.Cursor().Seek(pb)
		return nil
	}); err != nil {
		panic(err.Error())
	}

	if r != nil {
		rconn := newKcpconn(string(r))
		go io.Copy(rconn, iconn)
		io.Copy(iconn, rconn)
	} else {
		rconn, err := net.ResolveTCPAddr("tcp", string(r))
		if err != nil {

		}
		net.DialTCP("tcp", nil, rconn)

	}

}

func newKcpconn(raddr string) *kcp.UDPSession {
	return nil
}
