package server

import (
	"fmt"
	"itun/segment"
	"sync"

	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type SessionMgr struct {
	tcpMu sync.RWMutex
	tcp   map[uint16]relraw.RawConn

	udpMu sync.RWMutex
	udp   map[uint16]relraw.RawConn
}

type ErrUnsupportPoroto byte

func (eup ErrUnsupportPoroto) Error() string {
	return fmt.Sprintf("unsupport transport protocol %d", eup)
}

type ErrUnregistorSession uint16

func (eus ErrUnregistorSession) Error() string {
	return fmt.Sprintf("not registor session %d", eus)
}

func (sm *SessionMgr) Uplink(pxy segment.PxySeg, reserved int) error {
	id, proto := pxy.Get()

	switch proto {
	case header.TCPProtocolNumber:
		sm.tcpMu.RLock()
		conn, ok := sm.tcp[id]
		sm.tcpMu.RUnlock()
		if !ok {
			return ErrUnregistorSession(id)
		}

		return conn.WriteReservedIPHeader(pxy, reserved)

	case header.UDPProtocolNumber:
		sm.udpMu.RLock()
		conn, ok := sm.udp[id]
		sm.udpMu.RUnlock()
		if !ok {
			return ErrUnregistorSession(id)
		}

		return conn.WriteReservedIPHeader(pxy, reserved)
	default:
		return ErrUnsupportPoroto(proto)
	}
}

func (sm *SessionMgr) AddSession() error {

	go sm.downlink()

	return nil
}

// DelSession delete the session, also will be delete automatic exceed idle time limit
func (sm *SessionMgr) DelSession() error {
	return nil
}

// downlink every session correspond donwlink service
func (sm *SessionMgr) downlink() {

}
