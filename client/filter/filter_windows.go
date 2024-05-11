package filter

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun"
	"github.com/pkg/errors"

	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type filter struct {
	// default rule
	defaultEnable atomic.Bool
	def           *defaultFilter

	dnsEnable atomic.Bool

	// process rule
	processEnable atomic.Bool
	processes     []string
	processMu     sync.RWMutex
}

func newFilter() *filter {
	return &filter{
		def: NewDefaultFiler(2, time.Second*30),
	}
}

func (f *filter) Close() error { return nil }

func (f *filter) Hit(ip *packet.Packet) (bool, error) {
	id := fatun.FromIP(ip.Bytes())
	if id.Dst.Addr().IsLoopback() || id.Dst.Addr().IsMulticast() {
		return false, nil
	}

	if f.defaultEnable.Load() {
		switch id.Proto {
		case header.TCPProtocolNumber:
			var tcp header.TCP
			if id.Dst.Addr().Is4() {
				tcp = header.IPv4(ip.Bytes()).Payload()
			} else {
				tcp = header.IPv6(ip.Bytes()).Payload()
			}

			if f.def.Filter(id.Src, tcp.Flags()) {
				return true, nil
			}
		default:
		}
	}

	if f.dnsEnable.Load() {
		const dns = 53
		if id.Dst.Port() == dns {
			return true, nil
		}
	}

	if f.processEnable.Load() {
		name, err := Global.Name(id.Src, uint8(id.Proto))
		if err != nil {
			return false, err
		} else if name == "" {
			return false, errors.WithMessage(fatun.ErrNotRecord{}, id.String())
		}

		f.processMu.RLock()
		defer f.processMu.RUnlock()
		hit := slices.Contains(f.processes, name)
		return hit, nil
	}

	return false, nil
}

func (f *filter) Add(filter string) error {
	filter = strings.TrimSpace(filter)
	switch filter {
	case DefaultFilter:
		f.defaultEnable.Store(true)
	case DNSFilter:
		f.dnsEnable.Store(true)
	default: // process name
		f.processMu.Lock()
		defer f.processMu.Unlock()

		if !slices.Contains(f.processes, filter) {
			f.processes = append(f.processes, filter)
			f.processEnable.Store(true)
		}
	}

	return nil
}
func (f *filter) Del(filter string) error {
	filter = strings.TrimSpace(filter)
	switch filter {
	case DefaultFilter:
		f.defaultEnable.Store(false)
	case DNSFilter:
		f.dnsEnable.Store(false)
	default:
		f.processMu.Lock()
		defer f.processMu.Unlock()

		f.processes = slices.DeleteFunc(f.processes,
			func(s string) bool { return s == filter },
		)
		if len(f.processes) == 0 {
			f.processEnable.Store(false)
		}
	}
	return nil
}

func (f *filter) Filters() []string {
	f.processMu.RLock()
	defer f.processMu.RUnlock()

	fs := slices.Clone(f.processes)
	if f.defaultEnable.Load() {
		fs = append([]string{DefaultFilter}, fs...)
	}
	if f.dnsEnable.Load() {
		fs = append([]string{DNSFilter}, fs...)
	}
	return fs
}
