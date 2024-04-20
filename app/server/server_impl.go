package server

import (
	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/app/server/adapter"
)

type proxyerImpl Server

type proxyerImplPtr = *proxyerImpl

func (s *proxyerImpl) Config() *app.Config     { return s.cfg }
func (s *proxyerImpl) Adapter() *adapter.Ports { return s.ap }
