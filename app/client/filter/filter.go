package filter

import (
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
)

// Hitter validate the session is hit rule.
type Hitter interface {

	// todo: hit的传入参数应该是ip包本身
	// todo: hit应该返回三种状态：1.命中  2. 未命中  3. 未查询到（capture应该直接丢弃此数据包）

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
