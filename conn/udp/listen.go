package udp

import (
	"net"
	"net/netip"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/netkit/packet"
)

type Listen struct {
	conn *net.UDPConn
}

var _ conn.Listener = (*Listen)(nil)

func (l *Listen) Accept() (conn.Conn, error) {

	return nil, nil
}

func (l *Listen) Addr() netip.AddrPort {
	return netip.AddrPort{}
}
func (l *Listen) Close() error

type acceptConn struct {
	l     *Listen
	raddr netip.AddrPort

	buff []*packet.Packet
}

var _ udp = (*acceptConn)(nil)

func (c *acceptConn) Read([]byte) (int, error) {

	return 0, nil
}
func (c *acceptConn) Write(b []byte) (int, error) {
	return c.l.conn.WriteToUDPAddrPort(b, c.raddr)
}
func (c *acceptConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: c.l.Addr().Addr().AsSlice(), Port: int(c.l.Addr().Port())}
}
func (c *acceptConn) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: c.raddr.Addr().AsSlice(), Port: int(c.raddr.Port())}
}
func (c *acceptConn) Close() error {
	return nil
}
