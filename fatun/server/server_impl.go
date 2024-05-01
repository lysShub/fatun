//go:build linux
// +build linux

package server

import (
	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/proxyer"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) Config() *fatun.Config { return s.cfg }

func (s *proxyerImpl) AddSession(sess session.Session, pxy proxyer.IProxyer) error {
	err := s.m.Add(sess, pxy)
	return err
}

func (s *proxyerImpl) Send(sess session.Session, pkt *packet.Packet) error {
	return (*Server)(s).send(sess, pkt)
}
