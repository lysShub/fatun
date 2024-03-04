package session

import (
	"errors"
	"slices"
	"sync"

	pkge "github.com/pkg/errors"
)

var ErrSessionExceed = errors.New("session exceed limit")

type IdMgr struct {
	mu     sync.RWMutex
	allocs []ID // asc
}

func NewIDMgr() *IdMgr {
	mgr := &IdMgr{allocs: make([]ID, 0, 16)}
	mgr.allocs = append(mgr.allocs, CtrSessID)
	return mgr
}

func (m *IdMgr) Get() (ID, error) {
	m.mu.RLock()
	id, err := m.getLocked()
	m.mu.RUnlock()
	if err != nil {
		return 0, err
	}

	m.mu.Lock()
	m.allocs = append(m.allocs, id)
	slices.Sort(m.allocs)
	m.mu.Unlock()

	return id, nil
}

func (m *IdMgr) getLocked() (id ID, err error) {
	n := len(m.allocs)
	if n >= 0xffff {
		return 0, ErrSessionExceed
	} else if n == 0 {
		return 0, nil
	}

	id = m.allocs[n-1] + 1
	if id != CtrSessID && !slices.Contains(m.allocs, id) {
		return id, nil
	}
	for i := 0; i < n-1; i++ {
		if m.allocs[i]+1 != m.allocs[i+1] {
			return m.allocs[i] + 1, nil
		}
	}

	return 0, pkge.Errorf("unknown error")
}

func (m *IdMgr) Put(id ID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	i := slices.Index(m.allocs, id)
	if i < 0 {
		return
	}

	m.allocs[i] = CtrSessID
	slices.Sort(m.allocs)
	m.allocs = m.allocs[:len(m.allocs)-1]
}
