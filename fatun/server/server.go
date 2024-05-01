//go:build linux
// +build linux

package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"sync"

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

	// server keepalive
	// 1. snd/rcv map 的keepalive
	// 2. rcvVal 支持ref inc/dce, 当cnt为0是， proxyer将关闭自己（accpet时需要add spec Session）
	tcpSnder *net.IPConn
	udpSnder *net.IPConn
	sndMap   map[session.Session]uint16 // {clinet-addr,server-addr} : local-port
	sndMu    sync.RWMutex
	rcvMap   map[session.Session]rcvVal // {server-addr, proxyer-addr} : Proxyer
	rcvMu    sync.RWMutex

	laddrChecksum uint16
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

		laddrChecksum: checksum.Checksum(l.Addr().Addr().AsSlice(), 0),
	}

	s.tcpSnder, err = net.ListenIP("ip:tcp", &net.IPAddr{IP: l.Addr().Addr().AsSlice()})
	if err != nil {
		return nil, errors.WithStack(err)
	} else {
		err = FilterLocalPorts(s.tcpSnder, l.Addr().Port())
		if err != nil {
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
			// don't log, has too many other process's packet
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

	switch sess.Proto {
	case header.TCPProtocolNumber:
		t := header.TCP(pkt.Bytes())
		t.SetSourcePort(pxyPort)
		sum := checksum.Combine(t.Checksum(), pxyPort)
		sum = checksum.Combine(sum, s.laddrChecksum)
		t.SetChecksum(^sum)
		if debug.Debug() {
			psum := header.PseudoHeaderChecksum(
				sess.Proto,
				tcpip.AddrFrom4(s.l.Addr().Addr().As4()),
				tcpip.AddrFrom4(sess.Dst.Addr().As4()),
				0,
			)
			test.ValidTCP(test.T(), pkt.Bytes(), psum)
		}

		_, err := s.tcpSnder.WriteToIP(pkt.Bytes(), &net.IPAddr{IP: sess.Dst.Addr().AsSlice()})
		return errors.WithStack(err)
	case header.UDPProtocolNumber:
		udp := header.UDP(pkt.Bytes())
		udp.SetSourcePort(pxyPort)
		sum := checksum.Combine(udp.Checksum(), pxyPort)
		sum = checksum.Combine(sum, s.laddrChecksum)
		udp.SetChecksum(^sum)
		if debug.Debug() {
			psum := header.PseudoHeaderChecksum(
				sess.Proto,
				tcpip.AddrFrom4(s.l.Addr().Addr().As4()),
				tcpip.AddrFrom4(sess.Dst.Addr().As4()),
				0,
			)
			test.ValidTCP(test.T(), pkt.Bytes(), psum)
		}

		_, err := s.udpSnder.WriteToIP(pkt.Bytes(), &net.IPAddr{IP: sess.Dst.Addr().AsSlice()})
		return errors.WithStack(err)
	default:
		return errors.Errorf("not support transport protocol %d", sess.Proto)
	}
}
