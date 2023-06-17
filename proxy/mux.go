package proxy

import (
	"encoding/binary"
	"fmt"
	"itun/pack"
	"itun/proxy/maps"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/bpf"
	"golang.org/x/net/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type ProxyConn interface {
	ReadFrom([]byte) (int, netip.AddrPort, error)
	WriteTo([]byte, netip.AddrPort) (int, error)
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Close() error
}

type mux struct {
	// encode/decode pack
	Pack pack.Pack

	// proxy conn, used to send/recv client data
	ProxyConn

	locIP          netip.Addr
	pxyMap         *maps.Map
	rawTCP, rawUDP *net.IPConn // write only
}

func (s *mux) ListenAndServer() {
	var (
		b = make([]byte, 1532)

		n       int
		src     netip.AddrPort
		dst     netip.Addr // transport-layer proxy
		dstPort uint16
		udpHdr  header.UDP
		tcpHdr  header.TCP
		err     error
		proto   pack.Proto
		locPort uint16
	)

	newPort := false
	for {
		b = b[:cap(b)]
		n, src, err = s.ProxyConn.ReadFrom(b)
		if err != nil {
			panic(err)
		}

		n, proto, dst = s.Pack.Decode(b[:n])
		if n < 4 {
			continue
		}
		b = b[:n]
		dstPort = binary.BigEndian.Uint16(b[2:4])

		switch proto {
		case pack.TCP:
			locPort, newPort, err = s.pxyMap.UpGetTCP(src, netip.AddrPortFrom(dst, dstPort))
			if err != nil {
				panic(err)
			} else if newPort {
				go s.recvTCP(locPort)
			}

			tcpHdr = header.TCP(b)
			tcpHdr.SetSourcePortWithChecksumUpdate(locPort)
			_, err = s.rawTCP.WriteToIP(b, &net.IPAddr{IP: dst.AsSlice(), Zone: dst.Zone()})
			if err != nil {
				panic(err)
			}
		case pack.UDP:
			locPort, newPort, err = s.pxyMap.UpGetUDP(src, netip.AddrPortFrom(dst, dstPort))
			if err != nil {
				panic(err)
			} else if newPort {
				go s.recvUDP(locPort)
			}

			udpHdr = header.UDP(b)
			udpHdr.SetSourcePortWithChecksumUpdate(locPort)
			_, err = s.rawUDP.WriteToIP(b, &net.IPAddr{IP: dst.AsSlice(), Zone: dst.Zone()})
			if err != nil {
				panic(err)
			}
		case pack.ICMP:
			panic("icmp not support yet") // icmp need record sequence number
		default:
			panic(fmt.Sprintf("unknown proto %d", proto))
		}
	}
}

func (s *mux) recvTCP(locPort uint16) {
	var rc *ipv4.RawConn
	{ // TODO: use std IPConn and by SyscallConn to set BPF filter
		conn, err := net.ListenIP("ip4:"+pack.TCP.String(), &net.IPAddr{IP: s.locIP.AsSlice()})
		if err != nil {
			panic(err)
		}
		if rc, err = ipv4.NewRawConn(conn); err != nil {
			panic(err)
		}
		var locPortFilter = []bpf.Instruction{
			bpf.LoadMemShift{Off: 0},
			bpf.LoadIndirect{Off: 2, Size: 2},
			bpf.JumpIf{
				Cond:     bpf.JumpEqual,
				Val:      uint32(locPort),
				SkipTrue: 1,
			},
			bpf.RetConstant{Val: 0},
			bpf.RetConstant{Val: 0xffff},
		}
		filter, err := bpf.Assemble(locPortFilter)
		if err != nil {
			panic(err)
		}
		if err = rc.SetBPF(filter); err != nil {
			panic(err)
		}
	}

	var (
		b       = make([]byte, 1532)
		n       int
		hdr     header.IPv4
		hdrLen  int
		has     bool
		src     netip.AddrPort
		sAddr   netip.Addr
		dstPort uint16
		err     error
	)
	for {
		b = b[:cap(b)]
		n, err = rc.Read(b)
		if err != nil {
			panic(err)
		}
		b = b[:n]

		hdr = header.IPv4(b)
		hdrLen = int(hdr.HeaderLength())
		sAddr = netip.AddrFrom4([4]byte([]byte(hdr.SourceAddress())))

		src, has = s.pxyMap.DownGetTCP(netip.AddrPortFrom(sAddr, dstPort), locPort)
		if !has {
			panic("not found")
		}

		b = s.Pack.Encode(b[hdrLen:], pack.TCP, sAddr)

		_, err = s.ProxyConn.WriteTo(b, src)
		if err != nil {
			panic(err)
		}
	}
}

func (s *mux) recvUDP(locPort uint16) {
	var rc *ipv4.RawConn
	{
		conn, err := net.ListenIP("ip4:"+pack.UDP.String(), &net.IPAddr{IP: s.locIP.AsSlice()})
		if err != nil {
			panic(err)
		}
		if rc, err = ipv4.NewRawConn(conn); err != nil {
			panic(err)
		}

		var locPortFilter = []bpf.Instruction{
			bpf.LoadMemShift{Off: 0},
			bpf.LoadIndirect{Off: 2, Size: 2},
			bpf.JumpIf{
				Cond:     bpf.JumpEqual,
				Val:      uint32(locPort),
				SkipTrue: 1,
			},
			bpf.RetConstant{Val: 0},
			bpf.RetConstant{Val: 0xffff},
		}
		filter, err := bpf.Assemble(locPortFilter)
		if err != nil {
			panic(err)
		}
		if err = rc.SetBPF(filter); err != nil {
			panic(err)
		}
	}

	var (
		b      = make([]byte, 1532)
		n      int
		ipHdr  header.IPv4
		hdrLen int
		has    bool
		cAddr  netip.AddrPort
		sAddr  netip.Addr
		sPort  uint16
		err    error
	)
	for {
		b = b[:cap(b)]
		n, err = rc.Read(b)
		if err != nil {
			panic(err)
		}
		b = b[:n]

		ipHdr = header.IPv4(b)
		hdrLen = int(ipHdr.HeaderLength())
		sAddr = netip.AddrFrom4([4]byte([]byte(ipHdr.SourceAddress())))
		sPort = header.UDP(b[hdrLen:]).SourcePort()

		cAddr, has = s.pxyMap.DownGetUDP(netip.AddrPortFrom(sAddr, sPort), locPort)
		if !has {
			panic("not found")
		}

		b = s.Pack.Encode(b[hdrLen:], pack.UDP, sAddr)

		_, err = s.ProxyConn.WriteTo(b, cAddr)
		if err != nil {
			panic(err)
		}
	}
}

func (s *mux) Close() (err error) {
	err = s.ProxyConn.Close()

	e := s.pxyMap.Clsoe()
	if e != nil && err == nil {
		err = e
	}

	e = s.rawTCP.Close()
	if e != nil && err == nil {
		err = e
	}

	e = s.rawUDP.Close()
	if e != nil && err == nil {
		err = e
	}

	return err
}

func toLittle(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

func toBig(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}
