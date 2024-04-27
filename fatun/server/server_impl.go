package server

import (
	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/fatun/server/adapter"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) Config() *fatun.Config   { return s.cfg }
func (s *proxyerImpl) Adapter() *adapter.Ports { return s.ap }
