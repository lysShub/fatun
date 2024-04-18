//go:build windows
// +build windows

package mapping

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

type mapping struct {
	handle *divert.Handle

	addrs      map[session.Session]elem
	addrsMu    sync.RWMutex
	addTrigger *sync.Cond

	closeErr atomic.Pointer[error]
}

type elem struct {
	pid  uint32
	name string
}

var _ Mapping = (*mapping)(nil)

func newMapping() (*mapping, error) {
	var m = &mapping{addrs: map[session.Session]elem{}}
	m.addTrigger = sync.NewCond(&m.addrsMu)

	var err error
	m.handle, err = divert.Open("true", divert.Socket, 0, divert.Sniff|divert.ReadOnly)
	if err != nil {
		return nil, err
	}

	go m.service()
	go m.trigger()
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

func (m *mapping) trigger() {
	for m.closeErr.Load() == nil {
		m.addTrigger.Broadcast()
		time.Sleep(time.Millisecond * 250)
	}
}

func (m *mapping) service() error {
	go func() {
		for i := 0; ; i++ {
			time.Sleep(time.Minute)

			fh, err := os.Create(fmt.Sprintf("mapping%02d.txt", i))
			if err != nil {
				panic(err)
			}

			m.addrsMu.Lock()
			for s, e := range m.addrs {
				v := fmt.Sprintln(s.String(), "		", e.name)
				fh.WriteString(v)
			}
			m.addrsMu.Unlock()
			fh.Close()

			os.Exit(0)
		}
	}()

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
		case divert.SocketBind, divert.SocketConnect, divert.SocketListen, divert.SocketAccept:
			pid := addr.Socket().ProcessId
			name, err := (&process.Process{Pid: int32(pid)}).Name()
			if err != nil {
				return m.close(errors.WithStack(err))
			}

			m.addrsMu.Lock()
			m.addrs[getsession(addr)] = elem{
				pid:  pid,
				name: name,
			}
			m.addrsMu.Unlock()
			m.addTrigger.Broadcast()
		case divert.SocketClose:
			m.addrsMu.Lock()
			delete(m.addrs, getsession(addr))
			m.addrsMu.Unlock()
		default:
			if addr.Timestamp == 0 && addr.Event == 0 {
				continue // todo: divert can't return null result
			}
			return m.close(errors.Errorf("divert Socket event %d", addr.Event))
		}
	}
}

func getsession(addr *divert.Address) session.Session {
	f := addr.Socket()
	return session.Session{
		SrcAddr: f.LocalAddr(), SrcPort: f.LocalPort,
		Proto:   itun.Proto(f.Protocol),
		DstAddr: f.RemoteAddr(), DstPort: f.RemotePort,
	}
}

// todo: 需要处理未指定IP等问题
func (m *mapping) get(s session.Session) elem {
	m.addrsMu.RLock()
	defer m.addrsMu.RUnlock()
	return m.addrs[s]
}

func (m *mapping) Name(s session.Session) (string, error) {
	if e := m.closeErr.Load(); e != nil {
		return "", *e
	}
	if name := m.get(s).name; name != "" {
		return name, nil
	}

	// wait period
	for i := 0; i < 4; i++ {
		m.addrsMu.Lock()
		m.addTrigger.Wait()
		m.addrsMu.Unlock()
		if name := m.get(s).name; name != "" {
			return name, nil
		}
	}
	return "", nil
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
