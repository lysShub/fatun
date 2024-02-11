package server

import (
	"fmt"
	"itun"
	"net/netip"
	"sync"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Session struct {
	Proto itun.Proto

	// proxy connect's destination address
	Server netip.AddrPort
}

/*
	设计这个的目的是为了复用端口

	打算replay也用relraw, 每个session对应一个replay-conn，上行需要根据id路由到对应的replay-conn， 下行直接读取即可。
	那么id可以使用proxy唯一的了



	todo: relraw bindLocal是可选项
*/

type PortAdapter struct {
	mgr *itun.PortMgr

	sync.RWMutex

	sess map[Session] /* localPort */ uint16

	port map[portKey]map[ /* servers */ netip.AddrPort]struct{}
}

type portKey struct {
	proto     itun.Proto
	loaclPort uint16
}

// NewPortAdapter for reuse local machine port, reduce port consume
// require: the same local port corresponding different server addresses
func NewPortAdapter(addr netip.Addr) *PortAdapter {
	return &PortAdapter{
		mgr:  itun.NewPortMgr(addr),
		sess: make(map[Session]uint16, 16),
		port: make(map[portKey]map[netip.AddrPort]struct{}, 16),
	}
}

// GetPort get a local machine port
func (a *PortAdapter) GetPort(s Session) (port uint16, err error) {
	if icmp(s.Proto) {
		return 0, nil
	}

	// try reuse alloced port,
	a.RLock()
	for k, v := range a.port {
		if k.proto == s.Proto {
			continue
		}

		_, has := v[s.Server]
		if !has {
			port = k.loaclPort
			break
		}
	}
	a.RUnlock()

	// alloc new port
	if port == 0 {
		switch s.Proto {
		case header.TCPProtocolNumber:
			port, err = a.mgr.GetTCPPort()
			if err != nil {
				return 0, err
			}
		case header.UDPProtocolNumber:
			port, err = a.mgr.GetUDPPort()
			if err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("unknown transport protocol code %d", s.Proto)
		}
	}

	// update map
	a.Lock()
	a.sess[s] = port

	a.port[portKey{
		proto:     s.Proto,
		loaclPort: port,
	}][s.Server] = struct{}{}
	a.Unlock()

	return port, nil
}

// todo: idle timeout delete
func (a *PortAdapter) DelPort(s Session) error {
	if icmp(s.Proto) {
		return nil
	}

	var port uint16
	a.RLock()
	port = a.sess[s]
	a.RUnlock()

	if port == 0 {
		return fmt.Errorf("PortAdapter not exist Session {Proto:%d, Server:%s}", s.Proto, s.Server)
	}

	pk := portKey{
		proto:     s.Proto,
		loaclPort: port,
	}
	notuse := false

	a.Lock()
	delete(a.sess, s)

	delete(a.port[pk], s.Server)

	if len(a.port[pk]) == 0 {
		delete(a.port, pk)
		notuse = true
	}
	a.Unlock()

	if notuse {
		switch s.Proto {
		case header.TCPProtocolNumber:
			return a.mgr.DelTCPPort(port)
		case header.UDPProtocolNumber:
			return a.mgr.DelUDPPort(port)
		default:
			return fmt.Errorf("unknown transport protocol code %d", s.Proto)
		}
	}
	return nil
}

func icmp(proto itun.Proto) bool {
	return proto == header.ICMPv4ProtocolNumber ||
		proto == header.ICMPv6ProtocolNumber
}
