package client

import (
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/divert"

	"net/netip"
)

func connectRaw(addr netip.AddrPort) (relraw.RawConn, error) {
	return divert.Connect(netip.AddrPortFrom(relraw.LocalAddr(), 0), addr)
}
