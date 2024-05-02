package filter

import (
	"sync"

	"github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
)

type Hitter interface {
	// Hit hit outbound ip packet
	Hit(ip *packet.Packet) (bool, error)
}

// todo: humanable syntax
type Filter interface {
	Add(filter string) error
	Del(filter string) error
	Filters() []string
}

const (
	// default filter rule, will hit tcp connection when send secondary  SYN
	DefaultFilter = "default"
	DNSFilter     = "dns"
)

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
		Global, err = process.New()
	})
	if err != nil {
		return nil, err
	}

	return newFilter(), nil
}

var (
	Global     process.Mapping
	GlobalOnce sync.Once
)
