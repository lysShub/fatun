package server

import (
	"fmt"
	"itun/pack"
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

	locIP netip.Addr
	// upMap *maps.UpMap
}

func (s *mux) listenUP() {
	var (
		b = make([]byte, 1532)

		n       int
		src     netip.AddrPort
		dst     netip.Addr // transport-layer proxy, so no port
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
		if n == 0 {
			continue
		}
		b = b[:n]

		switch proto {
		case pack.TCP:
			locPort, newPort, err = s.upMap.GetTCP(src)
			if err != nil {
				panic(err)
			} else if newPort {
				// add tcp raw recving goroutine

			}

			*srcPortPtr = toBig(locPort)
			// send to server

		case pack.UDP:
			locPort, newPort, err = s.upMap.GetUDP(src)
			if err != nil {
				panic(err)
			} else if newPort {
			}

			*srcPortPtr = toBig(locPort)

		case pack.ICMP:
			panic("icmp not support yet") // icmp need record sequence number
		default:
			panic(fmt.Sprintf("unknown proto %d", proto))
		}

		fmt.Println(dst)
	}
}

func (s *mux) recvTCP(locPort uint16) {
	var rc *ipv4.RawConn
	{
		conn, err := net.ListenIP("ip4:"+pack.TCP.String(), &net.IPAddr{IP: s.locIP.AsSlice()})
		if err != nil {
			panic(err)
		}
		if rc, err = ipv4.NewRawConn(conn); err != nil {
			panic(err)
		}
	}

	err := rc.SetControlMessage(ipv4.FlagSrc, true)
	if err != nil {
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

	var (
		b = make([]byte, 1532)
		n int
	)
	for {
		b = b[:cap(b)]
		n, err = rc.Read(b)
		if err != nil {
			panic(err)
		}

		hdr := header.IPv4(b)
		src := []byte(hdr.SourceAddress())

		b = s.Pack.Encode(b[:n-int(hdr.HeaderLength())], pack.TCP, netip.AddrFrom4([4]byte(src)))

		s.PxyConn.WriteTo(b)

	}

}

func (s *mux) recvUDP(locPort uint16) {

}

func toLittle(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

func toBig(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}
