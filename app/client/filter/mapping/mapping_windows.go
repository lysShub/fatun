//go:build windows
// +build windows

package mapping

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

type mapping struct {
	handle *divert.Handle

	addrs   map[session.Session]elem
	addrsMu sync.RWMutex

	closeErr atomic.Pointer[error]
}

type elem struct {
	pid  uint32
	name string
}

var _ Mapping = (*mapping)(nil)

func newMapping() (*mapping, error) {
	var m = &mapping{addrs: map[session.Session]elem{}}

	var err error
	m.handle, err = divert.Open("true", divert.Flow, 0, divert.Sniff|divert.ReadOnly)
	if err != nil {
		return nil, err
	}

	go m.service()
	return m, nil
}

func (m *mapping) close(cause error) error {
	if m.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if m.handle != nil {
			if err := m.handle.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			m.closeErr.Store(&cause)
		}
		return cause
	}
	return *m.closeErr.Load()
}

func (m *mapping) service() error {
	var addr = &divert.Address{}
	var err error
	for {
		addr.Timestamp = 0
		_, err = m.handle.Recv(nil, addr)
		if err != nil {
			if errors.Is(err, divert.ErrClosed{}) || errors.Is(err, divert.ErrShutdown{}) {
				return m.close(nil)
			}
			return m.close(err)
		}

		switch addr.Event {
		case divert.FlowEstablishd:
			pid := addr.Flow().ProcessId

			name, err := (&process.Process{Pid: int32(pid)}).Name()
			if err != nil {
				return m.close(errors.WithStack(err))
			}
			fmt.Println("add", name, func() string {
				if addr.Outbound() {
					return "outbound"
				}
				return "inbound"
			}())

			m.addrsMu.Lock()
			m.addrs[getsession(addr)] = elem{
				pid:  pid,
				name: name,
			}
			m.addrsMu.Unlock()
		case divert.FlowDeleted:
			m.addrsMu.Lock()
			delete(m.addrs, getsession(addr))
			m.addrsMu.Unlock()
		default:
			if addr.Timestamp == 0 && addr.Event == 0 {
				continue // todo: divert can't return null result
			}
			return m.close(errors.Errorf("divert flow event %d", addr.Event))
		}
	}
}

func getsession(addr *divert.Address) session.Session {
	f := addr.Flow()
	return session.Session{
		SrcAddr: f.LocalAddr(), SrcPort: f.LocalPort,
		Proto:   itun.Proto(f.Protocol),
		DstAddr: f.RemoteAddr(), DstPort: f.RemotePort,
	}
}

func (m *mapping) Name(s session.Session) (string, error) {
	if e := m.closeErr.Load(); e != nil {
		return "", *e
	}
	m.addrsMu.RLock()
	defer m.addrsMu.RUnlock()

	return m.addrs[s].name, nil
}

func (m *mapping) Pid(s session.Session) (uint32, error) {
	if e := m.closeErr.Load(); e != nil {
		return 0, *e
	}
	m.addrsMu.RLock()
	defer m.addrsMu.RUnlock()

	return m.addrs[s].pid, nil
}

func (m *mapping) Close() error { return m.close(nil) }
