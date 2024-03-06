package filter

import (
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
)

type Filter interface {
	ProxyCh() <-chan session.Session

	EnableDefaultRule() error
	DisableDefaultRule() error

	// todo: temp
	AddRule(proto itun.Proto, pname string) error

	// DelRule()error
}
