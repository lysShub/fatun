//go:build windows
// +build windows

package sender

import (
	"net/netip"

	"github.com/pkg/errors"

	"github.com/lysShub/itun"
)

func newSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (Sender, error) {
	return nil, errors.New("not support")
}
