package server

import (
	"encoding/binary"
	"fmt"
	"itun/pack"
	"itun/server/maps"
	"net"
	"net/netip"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/net/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type mux struct {
	// encode/decode pack
	Pack pack.Pack

	// proxy conn, used to send/recv client data
	PxyConn interface {
		ReadFrom([]byte) (int, netip.AddrPort, error)
		WriteTo([]byte, netip.AddrPort) (int, error)
	}

	locIP          netip.Addr
	pxyMap         *maps.Map
	rawTCP, rawUDP *net.IPConn // write only
}

func (s *mux) listenUP() {
	var (
		b = make([]byte, 1532)

		n       int
		src     netip.AddrPort
		dst     netip.Addr // transport-layer proxy
		dstPort uint16
		err     error
		proto   pack.Proto
		locPort uint16
	)

	srcPortPtr := (*uint16)(unsafe.Pointer(&b[0]))
	newPort := false
	for {
		b = b[:cap(b)]
		n, src, err = s.PxyConn.ReadFrom(b)
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

			*srcPortPtr = toBig(locPort)
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

			*srcPortPtr = toBig(locPort)
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
		dstAddr netip.Addr
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
		dstAddr = netip.AddrFrom4([4]byte([]byte(hdr.SourceAddress())))

		src, has = s.pxyMap.DownGetTCP(netip.AddrPortFrom(dstAddr, dstPort), locPort)
		if !has {
			panic("not found")
		}

		b = s.Pack.Encode(b[hdrLen:], pack.TCP, dstAddr)

		_, err = s.PxyConn.WriteTo(b, src)
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
		b       = make([]byte, 1532)
		n       int
		hdr     header.IPv4
		hdrLen  int
		has     bool
		src     netip.AddrPort
		dstAddr netip.Addr
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
		dstAddr = netip.AddrFrom4([4]byte([]byte(hdr.SourceAddress())))

		src, has = s.pxyMap.DownGetTCP(netip.AddrPortFrom(dstAddr, dstPort), locPort)
		if !has {
			panic("not found")
		}

		b = s.Pack.Encode(b[hdrLen:], pack.TCP, dstAddr)

		_, err = s.PxyConn.WriteTo(b, src)
		if err != nil {
			panic(err)
		}
	}
}

func toLittle(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

func toBig(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}
