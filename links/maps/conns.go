package maps

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun/peer"
)

type connManager[P peer.Peer] struct {
	conns  map[id]*refConn[P]
	connMu sync.RWMutex
}

func newConnManager[P peer.Peer]() *connManager[P] {
	return &connManager[P]{conns: map[id]*refConn[P]{}}
}

type id struct {
	src, dst netip.AddrPort
}

type refConn[P peer.Peer] struct {
	fatcp.Conn[P]
	refs atomic.Int32
}

func (c *connManager[P]) Inc(conn fatcp.Conn[P]) int32 {
	id := id{src: conn.LocalAddr(), dst: conn.RemoteAddr()}

	c.connMu.RLock()
	v, has := c.conns[id]
	c.connMu.RUnlock()
	if !has {
		v = &refConn[P]{Conn: conn}
		c.connMu.Lock()
		c.conns[id] = v
		c.connMu.Unlock()
	}

	return v.refs.Add(1)
}

func (c *connManager[P]) Dec(conn fatcp.Conn[P]) int32 {
	id := id{src: conn.LocalAddr(), dst: conn.RemoteAddr()}

	c.connMu.RLock()
	v, has := c.conns[id]
	c.connMu.RUnlock()

	if !has {
		return 0
	}
	return v.refs.Add(-1)
}
