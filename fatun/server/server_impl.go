//go:build linux
// +build linux

package server

import (
	"net/netip"

	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) Config() *fatun.Config { return s.cfg }
func (s *proxyerImpl) GetPort(proto tcpip.TransportProtocolNumber, srv netip.AddrPort) (port uint16, err error) {
	return s.ap.GetPort(proto, srv)
}

func (s *proxyerImpl) AddSession(sess session.Session, pxy interface {
	Downlink(*packet.Packet, session.ID) error
}) error {
	locPort, err := s.ap.GetPort(sess.Proto, sess.Dst)
	if err != nil {
		return err
	}

	s.sndMu.Lock()
	s.sndMap[sess] = locPort
	s.sndMu.Unlock()

	s.rcvMu.Lock()
	s.rcvMap[session.Session{
		Src:   sess.Dst,
		Proto: sess.Proto,
		Dst:   netip.AddrPortFrom(s.l.Addr().Addr(), locPort),
	}] = rcvVal{
		proxyer:    pxy,
		clientPort: sess.Src.Port(),
	}
	s.rcvMu.Unlock()

	return nil
}

func (s *proxyerImpl) Send(sess session.Session, pkt *packet.Packet) error {
	return (*Server)(s).send(sess, pkt)
}
