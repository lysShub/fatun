package client

import "github.com/lysShub/itun/protocol"

type Filter interface {
	ProxyCh() <-chan protocol.Session

	ResetRule(any) error
}
