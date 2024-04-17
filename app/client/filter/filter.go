package filter

import (
	"sync"

	"github.com/lysShub/itun/app/client/filter/mapping"
)

// Hitter validate the session is hit rule.
type Hitter interface {
	Hit(ip []byte) (bool, error)
}

type ErrNotRecord struct{}

func (ErrNotRecord) Error() string { return "filter not record" }

type Filter interface {
	Hitter

	// default filter rule, will hit tcp connection when send secondary  SYN
	EnableDefault() error
	DisableDefault() error

	AddProcess(process string) error
	DelProcess(process string) error
}

func New() (Filter, error) {
	var err error
	GlobalOnce.Do(func() {
		Global, err = mapping.New()
	})
	if err != nil {
		return nil, err
	}

	return newFilter(), nil
}

var (
	Global     mapping.Mapping
	GlobalOnce sync.Once
)
