package client

import (
	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/tcp/divert"

	"net/netip"
)

func connectRaw(addr netip.AddrPort) (rsocket.RawConn, error) {
	return divert.Connect(netip.AddrPortFrom(rsocket.LocalAddr(), 0), addr)
}
