package client

import "github.com/lysShub/itun"

type Filter interface {
	ProxyCh() <-chan itun.Session

	ResetRule(any) error
}
