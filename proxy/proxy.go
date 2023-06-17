package proxy

import (
	"itun/pack"
	"itun/proxy/maps"
	"net"
	"net/netip"
)

type Server struct{}

func ListenUDPServer(laddr *net.UDPAddr) (*mux, error) {
	pxyConn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, err
	}

	if laddr.IP == nil {
		laddr.IP = net.IPv4(127, 0, 0, 1)
	}
	locIP := netip.MustParseAddr(laddr.IP.String())
	var mux = &mux{
		Pack:      pack.New(),
		ProxyConn: newUDPConn(pxyConn),

		locIP: locIP,
	}
	mux.pxyMap, err = maps.NewMap(locIP)
	if err != nil {
		return nil, err
	}
	mux.rawTCP, err = net.ListenIP("ip4:tcp", &net.IPAddr{IP: laddr.IP})
	if err != nil {
		return nil, err
	}
	mux.rawUDP, err = net.ListenIP("ip4:udp", &net.IPAddr{IP: laddr.IP})
	if err != nil {
		return nil, err
	}

	go func() {
		mux.ListenAndServer()
	}()

	return mux, nil
}

type udpConn struct {
	*net.UDPConn
}

var _ ProxyConn = (*udpConn)(nil)

func newUDPConn(c *net.UDPConn) *udpConn {
	return &udpConn{c}
}

func (c *udpConn) ReadFrom(b []byte) (int, netip.AddrPort, error) {
	return c.ReadFromUDPAddrPort(b)
}
func (c *udpConn) WriteTo(b []byte, addr netip.AddrPort) (int, error) {
	return c.WriteToUDPAddrPort(b, addr)
}
