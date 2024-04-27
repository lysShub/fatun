//go:build windows
// +build windows

package sender

import (
	"net/netip"

	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
)

func newSender(loc netip.AddrPort, proto tcpip.TransportProtocolNumber, dst netip.AddrPort) (Sender, error) {
	return nil, errors.New("not support")
}
