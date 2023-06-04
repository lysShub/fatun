package server

import (
	"errors"
	"itun/pack"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type connect struct {
	m         *sync.RWMutex
	svc       *serverMux
	proxyConn net.Conn // up-link

	srcMap  map[src]int32 // src:idx, idx is ses's index
	srcLock *sync.RWMutex

	pack pack.Pack

	ses []session

	mgrTimer *time.Timer

	done atomic.Bool
	err  error
}

// a session has unique src/dst
type src struct {
	proto   uint8
	srcPort uint16
}

func Connect(svc *serverMux, conn net.Conn) *connect {
	var u = &connect{
		svc:       svc,
		proxyConn: conn,
		srcMap:    map[src]int32{},
		srcLock:   &sync.RWMutex{},

		mgrTimer: time.NewTimer(time.Second * 5),

		m: &sync.RWMutex{},
	}

	go u.process()
	go u.monitor()
	return u
}

func (c *connect) process() {
	var (
		b       []byte = make([]byte, 1536)
		n       int
		err     error
		proto   uint8
		dstIP   netip.Addr
		srcPort *uint16 = (*uint16)(unsafe.Pointer(&b[0])) // big endian
		dstPort *uint16 = (*uint16)(unsafe.Pointer(&b[2])) // big endian
		idx     int32
	)

	for !c.done.Load() {
		b = b[:cap(b)]
		if n, err = c.proxyConn.Read(b); err != nil {
			c.Close(err)
		} else {

			n, proto, dstIP = c.pack.Decode(b[:n])

			idx = c.getIdx(src{proto, toLittle(*srcPort)})
			if idx < 0 || !c.ses[idx].idle() {
				idx, err = c.newSession(proto, toLittle(*srcPort), toLittle(*dstPort), dstIP)
				if err != nil {
					c.Close(err)
				} else {
					go c.ses[idx].Capture()
				}
			}
			*srcPort = toBig(c.ses[idx].locPort)

			_, err = c.svc.WriteTo(proto, b[:n], dstIP)
			if err != nil {
				c.Close(err)
			}
		}
	}
}

func (c *connect) newSession(proto uint8, srcPort, dstPort uint16, dstIP netip.Addr) (idx int32, err error) {
	if c.done.Load() {
		return -1, errors.New("connect is closed")
	}

	idx = -1
	for i := 0; i < len(c.ses); i++ {
		if c.ses[i].idle() {
			idx = int32(i)
			break
		}
	}
	if idx == -1 {
		c.ses = append(c.ses, session{})
		idx = 0
	}

	c.ses[idx].proto = proto
	c.ses[idx].srcPort = srcPort
	c.ses[idx].dstAddr = netip.AddrPortFrom(dstIP, dstPort)
	c.ses[idx].locPort, err = c.svc.GetPort(proto, c.ses[idx].dstAddr)
	if err != nil {
		return -1, err
	}

	c.srcLock.Lock()
	c.srcMap[src{proto, srcPort}] = idx
	c.srcLock.Unlock()

	return idx, nil
}

// getIdx 获取session, 只要
func (c *connect) getIdx(s src) (idx int32) {
	has := false
	c.srcLock.RLock()
	idx, has = c.srcMap[s]
	c.srcLock.RUnlock()
	if has {
		return idx
	} else {
		return -1
	}
}

func (c *connect) monitor() {
	var idleCon int
	for range c.mgrTimer.C {
		select {
		case <-c.svc.Done():
			c.Close(c.svc.Err()) // close by service
		default:
		}

		// check session
		c.m.RLock()
		n := len(c.ses)
		c.m.RUnlock()
		for i := 0; i < n; i++ {
			kill := c.ses[i].check()
			if kill {
				timeout := errors.New("session timeout")
				c.ses[i].close(timeout)
			}
		}

		// check connect
		workings := 0
		for i := 0; i < n; i++ {
			if !c.ses[i].idle() {
				workings++
			}
		}
		if workings == 0 {
			idleCon++
			if idleCon >= 5 {
				c.Close(errors.New("connect timeout"))
			}
		} else {
			idleCon = 0
		}
	}
}

func (c *connect) Close(err error) {
	if c.done.CompareAndSwap(false, true) {
		c.m.Lock()
		defer c.m.Unlock()

		c.err = err
		c.proxyConn.Close()
		c.mgrTimer.Stop()
		for i := 0; i < len(c.ses); i++ {
			c.svc.DelPort(c.ses[i].proto, c.ses[i].dstAddr, c.ses[i].locPort)
			c.ses[i].close(err)
		}
	}
}

func toLittle(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

func toBig(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}
