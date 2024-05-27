package udp

import "github.com/lysShub/fatun/conn"

type Conn struct {
	conn.Conn
}

var _ conn.Conn = (*Conn)(nil)
