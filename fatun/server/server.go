//go:build linux
// +build linux

package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/lysShub/fatun/fatun"
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
	cfg           *fatun.Config
	logger        *slog.Logger
	laddrChecksum uint16

	l *sconn.Listener

	tcpSnder *net.IPConn
	udpSnder *net.IPConn

	m *ttlmap
}

func NewServer(l *sconn.Listener, cfg *fatun.Config) (*Server, error) {
	var err error
	var s = &Server{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("server").WithAttrs([]slog.Attr{
			{Key: "addr", Value: slog.StringValue(l.Addr().String())},
		})),
		l:             l,
		laddrChecksum: checksum.Checksum(l.Addr().Addr().AsSlice(), 0),

		m: NewTTLMap(time.Minute*2, l.Addr().Addr()),
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
		p, clientPort, has := s.m.Downlink(sess)
		if !has {
			// don't log, has too many other process's packet
		} else {
			switch sess.Proto {
			case header.TCPProtocolNumber:
				header.TCP(pkt.Bytes()).SetDestinationPort(clientPort)
			case header.UDPProtocolNumber:
				header.UDP(pkt.Bytes()).SetDestinationPort(clientPort)
			default:
				panic("")
			}

			err = p.Downlink(pkt, session.ID{Remote: sess.Src.Addr(), Proto: sess.Proto})
			if err != nil {
				return s.close(err)
			}
		}
	}
}

func (s *Server) send(sess session.Session, pkt *packet.Packet) error {
	localPort, has := s.m.Uplink(sess)
	if !has {
		return fatun.ErrNotRecord{}
	}

	switch sess.Proto {
	case header.TCPProtocolNumber:
		t := header.TCP(pkt.Bytes())
		t.SetSourcePort(localPort)
		sum := checksum.Combine(t.Checksum(), localPort)
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
		udp.SetSourcePort(localPort)
		sum := checksum.Combine(udp.Checksum(), localPort)
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
