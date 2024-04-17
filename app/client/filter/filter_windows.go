package filter

import (
	"encoding/hex"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
	"github.com/pkg/errors"
)

type filter struct {
	count atomic.Int32

	// default rule
	defaultEnable atomic.Bool
	syncMap       map[session.Session]uint8 // record tcp conn sync count
	defaultMu     sync.RWMutex

	// process rule
	processEnable atomic.Bool
	processes     []string
	processMu     sync.RWMutex
}

func newFilter() *filter {
	return &filter{
		syncMap: map[session.Session]uint8{},
	}
}

func (f *filter) Hit(ip []byte) (bool, error) {
	f.count.Add(1)
	s := session.FromIP(ip)
	if !s.IsValid() {
		session.FromIP(ip)
		return false, errors.Errorf("capture invalid ip packet: %s", hex.EncodeToString(ip))
	}

	if f.defaultEnable.Load() {
		const count = 3
		if s.Proto == itun.TCP {
			f.defaultMu.Lock()
			// todo: require filter is capture-operate(not sniff-operate)
			// todo：should add SEQ identify tcp connection
			old := f.syncMap[s]
			f.syncMap[s] = old + 1
			n := len(f.syncMap)
			f.defaultMu.Unlock()

			// delete syncMap closed tcp connect
			if n > 128 && f.count.Load()%128 == 0 {
				f.cleanMap()
			}
			if old+1 >= count {
				return true, nil
			}
		}
	}

	if f.processEnable.Load() {
		name, err := Global.Name(s)
		if err != nil {
			return false, err
		} else if name == "" {
			return false, errors.WithStack(ErrNotRecord{})
		}

		f.processMu.RLock()
		defer f.processMu.RUnlock()
		return slices.Contains(f.processes, name), nil
	}

	return false, nil
}

func (f *filter) cleanMap() {
	f.defaultMu.Lock()
	defer f.defaultMu.Unlock()

	for s := range f.syncMap {
		if pid, err := Global.Pid(s); err != nil {
			return
		} else if pid == 0 {
			delete(f.syncMap, s)
		}
	}
}

func (f *filter) EnableDefault() error {
	f.defaultEnable.Store(true)
	return nil
}
func (f *filter) DisableDefault() error {
	f.defaultEnable.Store(false)
	return nil
}
func (f *filter) AddProcess(process string) error {
	f.processMu.Lock()
	defer f.processMu.Unlock()

	if !slices.Contains(f.processes, process) {
		f.processes = append(f.processes, process)
		f.processEnable.Store(true)
	}
	return nil
}
func (f *filter) DelProcess(process string) error {
	f.processMu.Lock()
	defer f.processMu.Unlock()

	f.processes = slices.DeleteFunc(f.processes,
		func(s string) bool { return s == process },
	)
	if len(f.processes) == 0 {
		f.processEnable.Store(false)
	}
	return nil
}
