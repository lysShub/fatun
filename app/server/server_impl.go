package server

import (
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server/adapter"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) Config() *app.Config     { return s.cfg }
func (s *proxyerImpl) Adapter() *adapter.Ports { return s.ap }
