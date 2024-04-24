//go:build linux
// +build linux

package session

import "github.com/lysShub/fatun/session"

func newSession(client Client, id session.ID, firstPack []byte) (Session, error) {
	panic("todo")
}
