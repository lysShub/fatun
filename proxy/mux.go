package proxy

import (
	"encoding/binary"
	"itun/pack"
	"itun/proxy/maps"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
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
	LocalAddr() net.Addr
	Close() error
}

type mux struct {
	// encode/decode pack
	pack.Pack

	// proxy conn, used to send/recv client data
	ProxyConn

	logger *zap.Logger

	locIP          netip.Addr
	pxyMap         *maps.Map
	rawTCP, rawUDP *net.IPConn // write only
	closed         atomic.Bool
}

func (s *mux) Prxoy() {
	s.logger.Info("proxy",
		zap.String("proxy proto", s.ProxyConn.LocalAddr().Network()),
		zap.String("listen ip", s.locIP.String()),
	)

	var (
		b = make([]byte, 1532)

		n       int
		cAddr   netip.AddrPort
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
		n, cAddr, err = s.ProxyConn.ReadFrom(b)
		if err != nil {
			if s.closed.Load() {
				return
			}
			s.logger.Panic("read from proxy conn", zap.Error(err))
		}

		n, proto, dst = s.Pack.Decode(b[:n])
		if n < 4 {
			continue
		}
		b = b[:n]
		dstPort = binary.BigEndian.Uint16(b[2:4])

		switch proto {
		case pack.TCP:
			locPort, newPort, err = s.pxyMap.UpGetTCP(cAddr, netip.AddrPortFrom(dst, dstPort))
			if err != nil {
				s.logger.Panic("get tcp port", zap.Error(err))
			} else if newPort {
				s.logger.Info("new tcp port", zap.Uint16("port", locPort))
				go s.recvTCP(locPort)
			}

			tcpHdr = header.TCP(b)
			tcpHdr.SetSourcePortWithChecksumUpdate(locPort)
			_, err = s.rawTCP.WriteToIP(b, &net.IPAddr{IP: dst.AsSlice(), Zone: dst.Zone()})
			if err != nil {
				s.logger.Panic("write to raw tcp", zap.Error(err))
			}
		case pack.UDP:
			locPort, newPort, err = s.pxyMap.UpGetUDP(cAddr, netip.AddrPortFrom(dst, dstPort))
			if err != nil {
				s.logger.Panic("get udp port", zap.Error(err))
			} else if newPort {
				s.logger.Info("new udp port", zap.Uint16("port", locPort))
				go s.recvUDP(locPort)
			}

			udpHdr = header.UDP(b)
			udpHdr.SetSourcePortWithChecksumUpdate(locPort)
			_, err = s.rawUDP.WriteToIP(b, &net.IPAddr{IP: dst.AsSlice(), Zone: dst.Zone()})
			if err != nil {
				s.logger.Panic("write to raw udp", zap.Error(err))
			}
		case pack.ICMP:
			// icmp need record sequence number
			s.logger.Warn("icmp not support yet", zap.String("client addr", cAddr.String()))
		default:
			s.logger.Warn("unknown proto",
				zap.Uint8("proto number", uint8(proto)),
				zap.String("client addr", cAddr.String()),
			)
		}
	}
}

func (s *mux) recvTCP(locPort uint16) {
	logger := s.logger.With(zap.Uint16("recv tcp", locPort))

	var rc *ipv4.RawConn
	{ // TODO: use std IPConn and by SyscallConn to set BPF filter
		conn, err := net.ListenIP("ip4:"+pack.TCP.String(), &net.IPAddr{IP: s.locIP.AsSlice()})
		if err != nil {
			logger.Error("listen ip", zap.Error(err))
			return
		}
		if rc, err = ipv4.NewRawConn(conn); err != nil {
			logger.Error("new raw conn", zap.Error(err))
			return
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
			logger.Error("build bpf assemble", zap.Error(err))
			return
		}
		if err = rc.SetBPF(filter); err != nil {
			logger.Error("set bpf", zap.Error(err))
			return
		}
	}

	var (
		b      = make([]byte, 1532)
		n      int
		hdr    header.IPv4
		hdrLen int
		has    bool
		src    netip.AddrPort
		sAddr  netip.Addr
		sPort  uint16
		err    error
	)
	for {
		b = b[:cap(b)]
		n, err = rc.Read(b)
		if err != nil {
			logger.Error("read from raw conn", zap.Error(err))
			return
		}
		b = b[:n]

		hdr = header.IPv4(b)
		hdrLen = int(hdr.HeaderLength())
		sAddr = netip.AddrFrom4([4]byte([]byte(hdr.SourceAddress())))

		src, has = s.pxyMap.DownGetTCP(netip.AddrPortFrom(sAddr, sPort), locPort)
		if !has {
			logger.Info("can't get client addr from proxy-map",
				zap.Stringer("server addr", netip.AddrPortFrom(sAddr, sPort)),
			)
			continue
		}

		b = s.Pack.Encode(b[hdrLen:], pack.TCP, sAddr)

		_, err = s.ProxyConn.WriteTo(b, src)
		if err != nil {
			logger.Error("write to proxy conn", zap.Error(err))
			return
		}
	}
}

func (s *mux) recvUDP(locPort uint16) {
	logger := s.logger.With(zap.Uint16("recv udp", locPort))

	var rc *ipv4.RawConn
	{
		conn, err := net.ListenIP("ip4:"+pack.UDP.String(), &net.IPAddr{IP: s.locIP.AsSlice()})
		if err != nil {
			logger.Error("listen ip", zap.Error(err))
			return
		}
		if rc, err = ipv4.NewRawConn(conn); err != nil {
			logger.Error("new raw conn", zap.Error(err))
			return
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
			logger.Error("build bpf assemble", zap.Error(err))
			return
		}
		if err = rc.SetBPF(filter); err != nil {
			logger.Error("set bpf", zap.Error(err))
			return
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
			logger.Error("read from raw conn", zap.Error(err))
			return
		}
		b = b[:n]

		ipHdr = header.IPv4(b)
		hdrLen = int(ipHdr.HeaderLength())
		sAddr = netip.AddrFrom4([4]byte([]byte(ipHdr.SourceAddress())))
		sPort = header.UDP(b[hdrLen:]).SourcePort()

		cAddr, has = s.pxyMap.DownGetUDP(netip.AddrPortFrom(sAddr, sPort), locPort)
		if !has {
			logger.Info("can't get client addr from proxy-map",
				zap.Stringer("server addr", netip.AddrPortFrom(sAddr, sPort)),
			)
			continue
		}

		b = s.Pack.Encode(b[hdrLen:], pack.UDP, sAddr)

		_, err = s.ProxyConn.WriteTo(b, cAddr)
		if err != nil {
			logger.Error("write to proxy conn", zap.Error(err))
			return
		}
	}
}

func (s *mux) Close() (err error) {
	s.closed.Store(true)

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
