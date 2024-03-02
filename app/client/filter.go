package client

import "github.com/lysShub/itun"

type Filter interface {
	ProxyCh() <-chan itun.Session

	EnableDefaultRule() error
	DisableDefaultRule() error

	// todo: temp
	AddRule(proto itun.Proto, pname string) error

	// DelRule()error
}
