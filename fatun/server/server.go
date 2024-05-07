//go:build linux
// +build linux

package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/proxyer"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/helper"
	"github.com/lysShub/rawsock/tcp"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func ListenAndServe(ctx context.Context, addr string, config *fatun.Config) error {
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
	l, err := sconn.NewListener(raw, config.Config)
	if err != nil {
		return err
	}

	s, err := NewServer(l, config)
	if err != nil {
		return err
	}

	return s.Serve(ctx)
}

type Server struct {
	config        *fatun.Config
	laddrChecksum uint16

	l *sconn.Listener

	tcpSnder *net.IPConn
	udpSnder *net.IPConn

	m *ttlmap

	closeErr atomic.Pointer[error]
}

func NewServer(l *sconn.Listener, config *fatun.Config) (*Server, error) {
	var err error
	var s = &Server{
		config:        config,
		l:             l,
		laddrChecksum: checksum.Checksum(l.Addr().Addr().AsSlice(), 0),

		m: NewTTLMap(time.Second*30, l.Addr().Addr()),
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
	if s.closeErr.CompareAndSwap(nil, &net.ErrClosed) {
		if s.l != nil {
			if err := s.l.Close(); err != nil {
				cause = err
			}
		}

		if s.tcpSnder != nil {
			if err := s.tcpSnder.Close(); err != nil {
				cause = err
			}
		}

		if s.udpSnder != nil {
			if err := s.udpSnder.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			if errorx.Temporary(cause) {
				s.config.Logger.Warn(cause.Error(), errorx.Trace(nil))
			} else {
				s.config.Logger.Error(cause.Error(), errorx.Trace(cause))
			}
			s.closeErr.Store(&cause)
		}
		return cause
	}
	return *s.closeErr.Load()
}

func (s *Server) Serve(ctx context.Context) error {
	s.config.Logger.Info("start", slog.String("addr", s.l.Addr().String()), slog.Bool("debug", debug.Debug()))

	for {
		conn, err := s.l.AcceptCtx(ctx)
		if err != nil {
			return err
		}

		s.config.Logger.Info("accept", "client", conn.RemoteAddr().String())
		go proxyer.ProxyAndServe(ctx, proxyerImplPtr(s), conn)
	}
}

func (s *Server) recvService(conn *net.IPConn) error {
	var pkt = packet.Make(s.config.MaxRecvBuffSize)

	for {
		n, err := conn.Read(pkt.Sets(fatun.Overhead, 0xffff).Bytes())
		if err != nil {
			if errorx.Temporary(err) {
				s.config.Logger.Warn(err.Error(), errorx.Trace(nil))
			}
			return s.close(err)
		} else if n < header.IPv4MinimumSize {
			continue
		}
		pkt.SetData(n)
		if _, err := helper.IntegrityCheck(pkt.Bytes()); err != nil {
			s.config.Logger.Warn(err.Error(), errorx.Trace(nil))
			continue
		}

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
				if !p.Closed() {
					s.config.Logger.Error(err.Error(), errorx.Trace(err))
				}

				// todo: send RST
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
