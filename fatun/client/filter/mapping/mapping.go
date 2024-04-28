package mapping

import (
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
)

// mapping of socket <==> process
type Mapping interface {
	Close() error

	Name(Endpoint) (string, error)
	Pid(Endpoint) (uint32, error)

	Pids() []uint32
	Names() []string
}

type LocalAddr struct {
	Proto tcpip.TransportProtocolNumber
	Addr  netip.AddrPort
}

func New() (Mapping, error) {
	return newMapping()
}

type Endpoint struct {
	Local netip.AddrPort                // local address
	Proto tcpip.TransportProtocolNumber // protocol
}

func (e Endpoint) Valid() bool {
	return e.Proto != 0 && e.Local.IsValid()
}
