//go:build linux
// +build linux

package fatun_test

import (
	"fmt"
	"testing"

	"golang.org/x/sys/unix"
)

func TestXxxx(t *testing.T) {

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, 0)

	fmt.Println(fd, err)
}
