//go:build windows
// +build windows

package ports

import (
	"errors"
	"syscall"
)

func (e *port) Close() error {
	e.m.Lock()
	defer e.m.Unlock()

	if len(e.dstAddrs) > 0 {
		return errors.New("can not close not null port")
	}
	if err := syscall.Close(syscall.Handle(e.fd)); err != nil {
		return err
	}
	e.port, e.fd = 0, 0
	return nil
}
