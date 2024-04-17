package mapping

import (
	"net/netip"

	"github.com/lysShub/itun/session"
	"gvisor.dev/gvisor/pkg/tcpip"
)

// mapping of socket <==> process
type Mapping interface {
	Name(session.Session) (string, error)
	Pid(session.Session) (uint32, error)
	Close() error
}

type LocalAddr struct {
	Proto tcpip.TransportProtocolNumber
	Addr  netip.AddrPort
}

func New() (Mapping, error) {
	return newMapping()
}
