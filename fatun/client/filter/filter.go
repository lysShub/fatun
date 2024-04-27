package filter

import (
	"sync"

	"github.com/lysShub/fatun/fatun/client/filter/mapping"
)

type Hitter interface {
	// Hit filter outbound ip packet
	Hit(ip []byte) (bool, error)
}

// todo: humanable syntax
type Filter interface {
	// default filter rule, will hit tcp connection when send secondary  SYN
	EnableDefault() error
	DisableDefault() error

	AddProcess(process string) error
	DelProcess(process string) error
	Processes() []string
}

type HitFilter interface {
	Hitter
	Filter
	Close() error
}

type ErrNotRecord struct{}

func (ErrNotRecord) Error() string   { return "filter not record" }
func (ErrNotRecord) Temporary() bool { return true }

func New() (HitFilter, error) {
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
