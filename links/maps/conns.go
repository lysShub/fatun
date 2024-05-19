package maps

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun/peer"
)

type connManager struct {
	conns  map[id]*refConn
	connMu sync.RWMutex
}

func newConnManager() *connManager {
	return &connManager{conns: map[id]*refConn{}}
}

type id struct {
	src, dst netip.AddrPort
}

type refConn struct {
	fatcp.Conn[peer.Peer]
	refs atomic.Int32
}

func (c *connManager) Inc(conn fatcp.Conn[peer.Peer]) int32 {
	id := id{src: conn.LocalAddr(), dst: conn.RemoteAddr()}

	c.connMu.RLock()
	v, has := c.conns[id]
	c.connMu.RUnlock()
	if !has {
		v = &refConn{Conn: conn}
		c.connMu.Lock()
		c.conns[id] = v
		c.connMu.Unlock()
	}

	return v.refs.Add(1)
}

func (c *connManager) Dec(conn fatcp.Conn[peer.Peer]) int32 {
	id := id{src: conn.LocalAddr(), dst: conn.RemoteAddr()}

	c.connMu.RLock()
	v, has := c.conns[id]
	c.connMu.RUnlock()

	if !has {
		return 0
	}
	return v.refs.Add(-1)
}
