package server

import (
	"itun/pack"
	"itun/server/ports"
	"net/netip"
)

type Server interface {
	ports.PortMgr

	WriteTo(prot pack.Proto, b []byte, dst netip.Addr) (int, error)
}
