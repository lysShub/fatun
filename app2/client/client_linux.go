package client

import (
	"net/netip"

	"github.com/lysShub/rsocket"
	"github.com/lysShub/rsocket/tcp/bpf"
)

func connectRaw(addr netip.AddrPort) (rsocket.RawConn, error) {
	return bpf.Connect(netip.AddrPortFrom(rsocket.LocalAddr(), 0), addr)
}
