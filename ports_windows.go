package itun

import "syscall"

func setFilterAll(conn interface {
	SyscallConn() (syscall.RawConn, error)
}) error {
	return nil
}
