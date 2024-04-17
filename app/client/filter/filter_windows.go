package filter

import (
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type filter struct {
	count atomic.Int32

	// default
	defaultEnable atomic.Bool
	syncMap       map[[2]addr]uint8 // record tcp conn sync count
	defaultMu     sync.RWMutex

	// process
	processMap    map[addr]bool // hited
	processes     []string
	processEnable atomic.Bool
	processMu     sync.RWMutex
}

func newFilter() *filter {
	return &filter{
		syncMap:    map[[2]addr]uint8{},
		processMap: map[addr]bool{},
	}
}

type addr struct {
	proto tcpip.TransportProtocolNumber
	addr  tcpip.Address
	port  uint16
}

func (f *filter) Hit(ip []byte) (bool, error) {
	f.count.Add(1)
	var (
		proto tcpip.TransportProtocolNumber
		iphdr header.Network
		hdr   header.Transport
	)
	switch header.IPVersion(ip) {
	case 4:
		iphdr = header.IPv4(ip)
	case 6:
		iphdr = header.IPv6(ip)
	default:
		return false, errors.WithStack(ErrNotRecord{})
	}
	proto = iphdr.TransportProtocol()
	switch proto {
	case header.TCPProtocolNumber:
		hdr = header.TCP(iphdr.Payload())
	case header.UDPProtocolNumber:
		hdr = header.UDP(iphdr.Payload())
	default:
		return false, errors.WithStack(ErrNotRecord{}) // todo: support icmp
	}

	if f.defaultEnable.Load() { // default
		const count = 3
		if iphdr.TransportProtocol() == header.TCPProtocolNumber {
			key := [2]addr{
				{addr: iphdr.SourceAddress(), port: hdr.SourcePort()},
				{addr: iphdr.SourceAddress(), port: hdr.SourcePort()},
			}
			f.defaultMu.Lock()
			old := f.syncMap[key]
			f.syncMap[key] = old + 1
			n := len(f.syncMap)
			f.defaultMu.Unlock()

			// delete syncMap closed tcp connect
			if n > 128 && f.count.Load()%128 == 0 {
				if err := f.updateConnectionMap(); err != nil {
					return false, err
				}
			}
			if old+1 >= count {
				return true, nil
			}
		}
	}

	if f.processEnable.Load() { // process
		addr := addr{
			proto: iphdr.TransportProtocol(),
			addr:  iphdr.SourceAddress(),
			port:  hdr.SourcePort(),
		}
		f.processMu.RLock()
		// 问题： 使用GetXxxExtenTable中的local有可能是为指定地址，纵使实际进程的socket已经bind了。
		hited, has := f.processMap[addr]
		f.processMu.RUnlock()
		if has {
			if hited {
				return true, nil
			}
			return false, nil
		}

		// validate new connection establish
		if err := f.updateConnectionMap(); err != nil {
			return false, err
		} else {
			f.processMu.RLock()
			hited, has := f.processMap[addr]
			f.processMu.RUnlock()
			if has {
				if hited {
					return true, nil
				}
				return false, nil
			} else {
				return false, errors.WithStack(ErrNotRecord{})
			}
		}
	}

	return false, nil
}

func (f *filter) updateConnectionMap() error {
	cs, err := net.Connections("all")
	if err != nil {
		return errors.WithStack(err)
	}

	if f.processEnable.Load() {
		ps, err := process.Processes()
		if err != nil {
			return err
		}

		var pids = map[int32]bool{}
		for _, e := range ps {
			if n, _ := e.Name(); slices.Contains(f.processes, n) {
				pids[e.Pid] = true
			}
		}

		f.processMu.Lock()
		clear(f.processMap)
		for _, e := range cs {
			addr := addr{
				proto: protoType(e.Type),
				addr:  tcpip.AddrFromSlice(netip.MustParseAddr(e.Laddr.IP).AsSlice()),
				port:  uint16(e.Laddr.Port),
			}
			f.processMap[addr] = pids[e.Pid]
		}
		f.processMu.Unlock()
	}

	if f.defaultEnable.Load() {
		var addrs = map[addr]struct{}{}
		for _, e := range cs {
			if protoType(e.Type) == header.TCPProtocolNumber {
				addrs[addr{
					proto: header.TCPProtocolNumber,
					addr:  tcpip.AddrFromSlice(netip.MustParseAddr(e.Laddr.IP).AsSlice()),
					port:  uint16(e.Laddr.Port),
				}] = struct{}{}
			}
		}

		f.defaultMu.Lock()
		for k := range f.syncMap {
			if _, has := addrs[k[0]]; !has {
				delete(f.syncMap, k)
			}
		}
		f.defaultMu.Unlock()
	}
	return nil
}

func protoType(typ uint32) tcpip.TransportProtocolNumber {
	switch typ {
	case syscall.SOCK_STREAM:
		return header.TCPProtocolNumber
	case syscall.SOCK_DGRAM:
		return header.UDPProtocolNumber
	default:
		return 0
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
