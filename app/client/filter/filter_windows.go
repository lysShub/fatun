package filter

import (
	"fmt"
	"net/netip"
	"sync"
	"syscall"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/session"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type filter struct {
	// tood: delete
	conns   map[session.Session]bool // hited
	connsMu sync.RWMutex
}

func NewFilter() (*filter, error) {
	f := &filter{
		conns: make(map[session.Session]bool, 16),
	}
	return f, nil
}

func (f *filter) Hit(s session.Session) bool {
	f.connsMu.RLock()
	_, ok := f.conns[s]
	f.connsMu.RUnlock()
	return ok
}

func (f *filter) HitOnce(s session.Session) bool {
	f.connsMu.RLock()
	hited, ok := f.conns[s]
	f.connsMu.RUnlock()
	if ok {
		return !hited
	}

	return false
}

func (f *filter) AddDefaultRule() error {
	return errors.New("not implement")
}

func (f *filter) DelDefaultRule() error {
	return errors.New("not implement")
}

func (f *filter) AddRule(process string, proto itun.Proto) error {
	go f.processService(process, proto)
	return nil
}

func (f *filter) DelRule(process string, proto itun.Proto) error {
	return errors.New("not implement")
}

func (f *filter) Close() error {
	return errors.New("not implement")
}

func (f *filter) processService(name string, proto itun.Proto) {

	ps, err := process.Processes()
	if err != nil {
		panic(err)
	}

	var pids []int32
	for _, e := range ps {
		if n, _ := e.Name(); n == name {
			pids = append(pids, e.Pid)
		}
	}

	if len(pids) == 0 {
		// todo: wait process start
		panic("wait process start")
	}

	fmt.Println(pids)

	for _, e := range pids {
		ns, err := net.ConnectionsPid("tcp4", e) // strings.ToLower(proto.String())
		if err != nil {
			continue
		}
		for _, n := range ns {
			s := toSession(n)

			f.connsMu.Lock()
			f.conns[s] = false // todo: validate
			f.connsMu.Unlock()
		}
	}
}

func toSession(stat net.ConnectionStat) session.Session {
	var p itun.Proto
	switch stat.Type {
	case syscall.SOCK_STREAM:
		p = itun.TCP
	case syscall.SOCK_DGRAM:
		p = itun.UDP
	default:
		panic("")
	}

	return session.Session{
		Src:   netip.AddrPortFrom(netip.MustParseAddr(stat.Laddr.IP), uint16(stat.Laddr.Port)),
		Proto: p,
		Dst:   netip.AddrPortFrom(netip.MustParseAddr(stat.Raddr.IP), uint16(stat.Raddr.Port)),
	}
}
