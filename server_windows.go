//go:build windows
// +build windows

package fatun

import (
	"errors"
	"net/netip"
)

func NewDefaultSender(laddr netip.AddrPort) ([]Sender, error) {
	return nil, errors.New("windows not default sender")
}
