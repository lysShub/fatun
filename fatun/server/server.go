//go:build linux
// +build linux

package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"unsafe"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/adapter"
	"github.com/lysShub/fatun/fatun/server/proxyer"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/conn/tcp"
	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/pkg/errors"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func ListenAndServe(ctx context.Context, addr string, cfg *fatun.Config) error {
	var laddr netip.AddrPort
	if addr, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return errors.WithStack(err)
	} else {
		ip := addr.IP
		if ip == nil {
			laddr = netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(addr.Port))
		} else if ip.To4() != nil {
			laddr = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ip.To4())), uint16(addr.Port))
		} else {
			laddr = netip.AddrPortFrom(netip.AddrFrom16([16]byte(ip.To16())), uint16(addr.Port))
		}
	}

	raw, err := tcp.Listen(laddr)
	if err != nil {
		return err
	}
	defer raw.Close()
	l, err := sconn.NewListener(raw, cfg.Config)
	if err != nil {
		return err
	}

	s, err := NewServer(l, cfg)
	if err != nil {
		return err
	}

	return s.Serve(ctx)
}

type Server struct {
	cfg    *fatun.Config
	logger *slog.Logger

	l *sconn.Listener

	ap *adapter.Ports

	tcpSnder *net.IPConn
	udpSnder *net.IPConn
	sndMap   map[session.Session]uint16 // {clinet-addr,server-addr} : proxyer-port
	sndMu    sync.RWMutex
	rcvMap   map[session.Session]rcvVal // {server-addr, proxyer-addr} : Proxyer
	rcvMu    sync.RWMutex
}

type rcvVal struct {
	proxyer interface {
		Downlink(*packet.Packet, session.ID) error
	}
	clientPort uint16
}

func NewServer(l *sconn.Listener, cfg *fatun.Config) (*Server, error) {
	var err error
	var s = &Server{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("server").WithAttrs([]slog.Attr{
			{Key: "addr", Value: slog.StringValue(l.Addr().String())},
		})),
		l:  l,
		ap: adapter.NewPorts(l.Addr().Addr()),

		sndMap: map[session.Session]uint16{},
		rcvMap: map[session.Session]rcvVal{},
	}

	s.tcpSnder, err = net.ListenIP("ip:tcp", &net.IPAddr{IP: l.Addr().Addr().AsSlice()})
	if err != nil {
		return nil, errors.WithStack(err)
	} else {
		if err = filter(s.tcpSnder); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	s.udpSnder, err = net.ListenIP("ip:udp", &net.IPAddr{IP: l.Addr().Addr().AsSlice()})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go s.recvService(s.tcpSnder)
	go s.recvService(s.udpSnder)
	return s, nil
}

func (s *Server) close(cause error) error {
	return cause
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("start")

	for {
		conn, err := s.l.AcceptCtx(ctx)
		if err != nil {
			return err
		}

		// p, err := proxyer.NewProxyer(proxyerImplPtr(s), conn)
		// if err != nil {
		// 	s.logger.Error(err.Error(), errorx.TraceAttr(err))
		// } else {
		// 	go func() {
		// 		err = p.Proxy(ctx)
		// 		if err != nil {
		// 			panic(err)
		// 		}
		// 	}()
		// }

		s.logger.Info("accepted", "client", conn.RemoteAddr().String())
		go proxyer.Proxy(ctx, proxyerImplPtr(s), conn)
	}
}

func (s *Server) recvService(conn *net.IPConn) error {
	var pkt = packet.Make(32, s.cfg.MTU)

	for {
		n, err := conn.Read(pkt.Sets(32, 0xffff).Bytes())
		if err != nil {
			return s.close(err)
		}
		pkt.SetData(n)

		sess := session.StripIP(pkt)
		s.rcvMu.RLock()
		p, has := s.rcvMap[sess]
		s.rcvMu.RUnlock()
		if !has {
			// todo: support skip ports
			// s.logger.Warn("proxy no found", "session", sess.String())
		} else {
			switch sess.Proto {
			case header.TCPProtocolNumber:
				header.TCP(pkt.Bytes()).SetDestinationPort(p.clientPort)
			case header.UDPProtocolNumber:
				header.UDP(pkt.Bytes()).SetDestinationPort(p.clientPort)
			default:
				panic("")
			}

			err = p.proxyer.Downlink(pkt, session.ID{Remote: sess.Src.Addr(), Proto: sess.Proto})
			if err != nil {
				return s.close(err)
			}
		}
	}
}

func (s *Server) send(sess session.Session, pkt *packet.Packet) error {
	s.sndMu.RLock()
	pxyPort, has := s.sndMap[sess]
	s.sndMu.RUnlock()
	if !has {
		return fatun.ErrNotRecord{}
	}

	// update port
	psum := header.PseudoHeaderChecksum(
		sess.Proto,
		tcpip.AddrFrom4(s.l.Addr().Addr().As4()),
		tcpip.AddrFrom4(sess.Dst.Addr().As4()),
		uint16(pkt.Data()),
	)
	switch sess.Proto {
	case header.TCPProtocolNumber:
		// header.TCP(pkt.Bytes()).SetSourcePortWithChecksumUpdate(pxyPort)

		// todo: optimize
		tcp := header.TCP(pkt.Bytes())
		tcp.SetSourcePort(pxyPort)
		tcp.SetChecksum(0)
		sum := checksum.Checksum(tcp, psum)
		tcp.SetChecksum(^sum)
	case header.UDPProtocolNumber:
		// header.UDP(pkt.Bytes()).SetSourcePortWithChecksumUpdate(pxyPort)

		tcp := header.TCP(pkt.Bytes())
		tcp.SetSourcePort(pxyPort)
		tcp.SetChecksum(0)
		sum := checksum.Checksum(tcp, psum)
		tcp.SetChecksum(^sum)
	default:
		panic("")
	}
	if debug.Debug() {
		test.ValidTCP(test.T(), pkt.Bytes(), checksum.Combine(psum, uint16(pkt.Data())))
	}

	_, err := s.tcpSnder.WriteToIP(pkt.Bytes(), &net.IPAddr{IP: sess.Dst.Addr().AsSlice()})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func filter(conn *net.IPConn) error {
	var ins = []bpf.Instruction{
		// load ip version to A
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 4},

		// ipv4
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 4, SkipTrue: 1},
		bpf.LoadMemShift{Off: 0},

		// ipv6
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 6, SkipTrue: 1},
		bpf.LoadConstant{Dst: bpf.RegX, Val: 40},
		/*
		  reg X ipHdrLen
		*/

		// dst-port not equal 443
		bpf.LoadIndirect{Off: header.TCPDstPortOffset, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(443), SkipTrue: 1},
		bpf.RetConstant{Val: 0},

		bpf.RetConstant{Val: 0xffff},
	}

	raw, err := conn.SyscallConn()
	if err != nil {
		return errors.WithStack(err)
	}

	var e error
	err = raw.Control(func(fd uintptr) {
		e = setBPF(fd, []bpf.Instruction{bpf.RetConstant{Val: 0}})
		if e != nil {
			return
		}
		var b = make([]byte, 1)
		for {
			n, _, _ := unix.Recvfrom(int(fd), b, unix.MSG_DONTWAIT)
			if n < 0 {
				break
			}
		}
		e = setBPF(fd, ins)
	})
	if err != nil {
		return errors.WithStack(err)
	} else if e != nil {
		return errors.WithStack(e)
	}

	return nil
}

func setBPF(fd uintptr, ins []bpf.Instruction) error {
	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(ins); err != nil {
		return err
	} else {
		prog = &unix.SockFprog{
			Len:    uint16(len(rawIns)),
			Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
		}
	}

	err := unix.SetsockoptSockFprog(
		int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog,
	)
	return err
}
