package proxy

import (
	"itun/pack"
	"itun/proxy/maps"
	"net"
	"net/netip"

	"go.uber.org/zap"
)

// type Proxy interface {
// 	WithProxyConn(ProxyConn) error
// 	WithLogPath(...string) error // zap.Config.OutputPaths
// 	WithProtos(...pack.Proto) error

// 	Prxoy()
// }

type Config struct {
	ListenAddr net.Addr

	// see zap.Open
	LogPath []string

	// proxy transport protocol: TCP, UDP
	Protos []pack.Proto
}

type proxy struct {
	*mux
}

func ListenAndProxyWithUDP(laddr *net.UDPAddr) (*proxy, error) {
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

	mux.logger, err = zap.NewProduction()
	if err != nil {
		return nil, err
	}

	mux.pxyMap, err = maps.NewMap(locIP)
	if err != nil {
		return nil, err
	}

	mux.rawTCP, err = net.ListenIP("ip4:tcp", &net.IPAddr{IP: laddr.IP})
	if err != nil {
		return nil, err
	}
	if err = mux.rawTCP.SetReadBuffer(0); err != nil {
		return nil, err
	}

	mux.rawUDP, err = net.ListenIP("ip4:udp", &net.IPAddr{IP: laddr.IP})
	if err != nil {
		return nil, err
	}
	if err = mux.rawUDP.SetReadBuffer(0); err != nil {
		return nil, err
	}

	go func() {
		mux.Prxoy()
	}()

	return &proxy{mux}, nil
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
