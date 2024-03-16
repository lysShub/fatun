//go:build windows
// +build windows

package filter

import (
	"sync"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type mock struct {
	name string

	hited   map[session.Session]bool
	hitedMu sync.RWMutex
}

func NewMock(process string) *mock {
	return &mock{name: process, hited: make(map[session.Session]bool)}
}

func (m *mock) AddDefaultRule() error {
	return errors.New("not implement")
}
func (m *mock) DelDefaultRule() error {
	return errors.New("not implement")
}
func (m *mock) AddRule(process string, proto itun.Proto) error {
	return errors.New("not implement")
}
func (m *mock) DelRule(process string, proto itun.Proto) error {
	return errors.New("not implement")
}
func (m *mock) Hit(s session.Session) bool {
	panic(errors.New("not implement"))
}
func (m *mock) HitOnce(s session.Session) bool {
	if s.Proto != itun.TCP {
		return false
	}

	ps, err := process.Processes()
	if err != nil {
		panic(err)
	}

	var pids []int32
	for _, e := range ps {
		if n, _ := e.Name(); n == m.name {
			pids = append(pids, e.Pid)
		}
	}

	if len(pids) == 0 {
		return false
	}

	for _, e := range pids {
		ns, err := net.ConnectionsPid("tcp4", e) // strings.ToLower(proto.String())
		if err != nil {
			continue
		}
		for _, n := range ns {
			ns := toSession(n)

			if ns == s {

				m.hitedMu.RLock()
				hited := m.hited[ns]
				m.hitedMu.RUnlock()

				if !hited {
					m.hitedMu.Lock()
					m.hited[ns] = true
					m.hitedMu.Unlock()
				}
				return !hited
			}
		}
	}
	return false
}
