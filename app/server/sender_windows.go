//go:build windows
// +build windows

package server

import (
	"errors"
	"net/netip"

	"github.com/lysShub/itun"
)

func newSender(loc netip.AddrPort, proto itun.Proto, dst netip.AddrPort) (Sender, error) {
	return nil, errors.New("not support")
}
