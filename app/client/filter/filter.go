package filter

import (
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
)

// Hitter validate the session is hit rule.
type Hitter interface {

	//
	Hit(s session.Session) bool

	// ？忘了这个和hit有啥区别？
	HitOnce(s session.Session) bool
}

type Filter interface {
	Hitter

	AddDefaultRule() error
	DelDefaultRule() error

	// todo: simple
	AddRule(process string, proto itun.Proto) error
	DelRule(process string, proto itun.Proto) error
}
