package proxy_test

import (
	"itun/pack"
	"itun/proxy"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestProxyUDP(t *testing.T) {
	var (
		sAddr  = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19986}
		pAddr  = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9721}
		cAddr1 = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34459}
		cAddr2 = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34460}
	)
	var packer = pack.New()

	// ping-pong server
	go func() {
		conn, err := net.ListenUDP("udp", sAddr)
		require.NoError(t, err)
		defer conn.Close()

		var buf = make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			require.NoError(t, err)

			_, err = conn.WriteToUDP(buf[:n], addr)
			require.NoError(t, err)
		}
	}()
	time.Sleep(time.Millisecond * 100)

	// proxyer, ProxyConn is a UDP conn
	{
		mux, err := proxy.ListenAndProxyWithUDP(pAddr)
		require.NoError(t, err)
		defer mux.Close()
	}

	// client1
	{
		c, err := net.DialUDP("udp", nil, pAddr)
		require.NoError(t, err)

		d := genUDP(cAddr1, sAddr, []byte("hello"))
		d = packer.Encode(d, pack.UDP, sAddr.AddrPort().Addr())
		_, err = c.Write(d)
		require.NoError(t, err)

		var buf = make([]byte, 1024)
		n, err := c.Read(buf)
		require.NoError(t, err)

		n, _, _ = packer.Decode(buf[:n])
		require.Equal(t, "hello", string(buf[8:n]))
	}

	// client2
	{
		c, err := net.DialUDP("udp", nil, pAddr)
		require.NoError(t, err)

		d := genUDP(cAddr2, sAddr, []byte("world"))
		d = packer.Encode(d, pack.UDP, sAddr.AddrPort().Addr())
		_, err = c.Write(d)
		require.NoError(t, err)

		var buf = make([]byte, 1024)
		n, err := c.Read(buf)
		require.NoError(t, err)

		n, _, _ = packer.Decode(buf[:n])
		require.Equal(t, "world", string(buf[8:n]))
	}
}

func TestProxyTCP(t *testing.T) {
	var (
		sAddr  = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19986}
		pAddr  = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9721}
		cAddr1 = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34459}
		cAddr2 = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34460}
	)
	var packer = pack.New()

	// ping-pong server
	go func() {
		listen, err := net.ListenTCP("tcp", sAddr)
		require.NoError(t, err)
		defer listen.Close()

		for {
			conn, err := listen.AcceptTCP()
			require.NoError(t, err)

			go func() {
				var buf = make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					require.NoError(t, err)

					_, err = conn.Write(buf[:n])
					require.NoError(t, err)
				}
			}()
		}
	}()
	time.Sleep(time.Millisecond * 100)

	// proxyer, ProxyConn is a UDP conn
	{
		mux, err := proxy.ListenAndProxyWithUDP(pAddr)
		require.NoError(t, err)
		defer mux.Close()
	}

}

func genUDP(
	srcAddr, dstAddr *net.UDPAddr,
	payload []byte,
) []byte {

	var d = header.UDP(make([]byte, 8, len(payload)+16))

	d.SetSourcePort(uint16(srcAddr.Port))
	d.SetDestinationPort(uint16(dstAddr.Port))
	d.SetLength(uint16(len(payload) + 8))
	d = append(d, payload...)

	s := header.PseudoHeaderChecksum(
		header.UDPProtocolNumber,
		tcpip.Address(srcAddr.IP),
		tcpip.Address(dstAddr.IP),
		uint16(len(payload)+8),
	)
	s = checksum.Checksum(d, s)
	d.SetChecksum(0xffff - s)

	ok := d.IsChecksumValid(
		tcpip.Address(srcAddr.IP),
		tcpip.Address(dstAddr.IP),
		checksum.Checksum(d[8:], 0),
	)
	if !ok {
		panic("")
	}

	return d
}
