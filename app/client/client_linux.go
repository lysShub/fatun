package client

import (
	"net/netip"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
)

func connectRaw(addr netip.AddrPort) (relraw.RawConn, error) {
	return bpf.Connect(netip.AddrPortFrom(relraw.LocalAddr(), 0), addr)
}
