//go:build linux
// +build linux

package fatun

import (
	"errors"
	"net/netip"
)

func NewDefaultCapture(laddr netip.AddrPort, process string, overhead int) (Capture, error) {
	return nil, errors.New("linux not default capture")
}
