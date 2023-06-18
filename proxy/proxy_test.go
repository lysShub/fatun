package proxy_test

import (
	"fmt"
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

func TestProxy(t *testing.T) {
	var saddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19986}
	var paddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9721}
	var caddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34459}
	var packer = pack.New()

	// ping-pong server
	go func() {
		conn, err := net.ListenUDP("udp", saddr)
		require.NoError(t, err)
		defer conn.Close()

		var buf = make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			require.NoError(t, err)

			fmt.Println("server recv:", string(buf[:n]))

			_, err = conn.WriteToUDP(buf[:n], addr)
			require.NoError(t, err)
		}
	}()
	time.Sleep(time.Millisecond * 100)

	// proxyer
	{
		mux, err := proxy.ListenAndProxyWithUDP(paddr)
		require.NoError(t, err)
		defer mux.Close()
	}

	// client
	{
		c, err := net.DialUDP("udp", nil, paddr)
		require.NoError(t, err)

		d := genUDP(caddr, saddr, []byte("hello"))
		d = packer.Encode(d, pack.UDP, saddr.AddrPort().Addr())
		_, err = c.Write(d)
		require.NoError(t, err)

		var buf = make([]byte, 1024)
		n, err := c.Read(buf)
		require.NoError(t, err)

		n, _, _ = packer.Decode(buf[:n])
		require.Equal(t, "hello", string(buf[8:n]))
	}
}

func TestProxy1(t *testing.T) {
	var saddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 19986}
	var paddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9721}
	var caddr1 = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34459}
	var caddr2 = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 34460}
	var packer = pack.New()

	// ping-pong server
	go func() {
		conn, err := net.ListenUDP("udp", saddr)
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

	// proxyer
	{
		mux, err := proxy.ListenAndProxyWithUDP(paddr)
		require.NoError(t, err)
		defer mux.Close()
	}

	// client1
	{
		c, err := net.DialUDP("udp", nil, paddr)
		require.NoError(t, err)

		d := genUDP(caddr1, saddr, []byte("hello"))
		d = packer.Encode(d, pack.UDP, saddr.AddrPort().Addr())
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
		c, err := net.DialUDP("udp", nil, paddr)
		require.NoError(t, err)

		d := genUDP(caddr2, saddr, []byte("world"))
		d = packer.Encode(d, pack.UDP, saddr.AddrPort().Addr())
		_, err = c.Write(d)
		require.NoError(t, err)

		var buf = make([]byte, 1024)
		n, err := c.Read(buf)
		require.NoError(t, err)

		n, _, _ = packer.Decode(buf[:n])
		require.Equal(t, "world", string(buf[8:n]))
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
