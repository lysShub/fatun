//go:build linux
// +build linux

package server

import (
	"log/slog"
	"net/netip"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/server/proxyer"
	"github.com/lysShub/netkit/packet"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) MaxRecvBuffSize() int { return s.config.MaxRecvBuffSize }
func (s *proxyerImpl) Logger() *slog.Logger { return s.config.Logger }
func (s *proxyerImpl) AddSession(sess fatun.Session, pxy proxyer.Proxyer) error {
	err := s.m.Add(sess, pxy)
	return err
}
func (s *proxyerImpl) Send(sess fatun.Session, pkt *packet.Packet) error {
	return (*Server)(s).send(sess, pkt)
}
func (s *proxyerImpl) Close(client netip.AddrPort) {
	// ttlkey add clientPort
}