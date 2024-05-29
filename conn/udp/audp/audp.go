package audp

// acceptable udp conn

import (
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type Conn interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close() error
}

type Listener struct {
	udp *net.UDPConn

	pool *sync.Pool

	connsMu sync.RWMutex
	conns   map[netip.AddrPort]*acceptConn

	closeErr errorx.CloseErr
}

func Listen(addr *net.UDPAddr, maxRecvSize int) (*Listener, error) {
	var l = &Listener{
		conns: map[netip.AddrPort]*acceptConn{},
	}

	var err error
	if l.udp, err = net.ListenUDP("udp", addr); err != nil {
		return nil, l.close(err)
	}

	l.pool = &sync.Pool{
		New: func() any {
			b := make([]byte, maxRecvSize)
			return segment(&b)
		},
	}

	return l, nil
}

func (l *Listener) close(cause error) error {
	l.connsMu.Lock()
	for _, e := range l.conns {
		e.destroy()
	}
	l.connsMu.Unlock()

	return l.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if l.udp != nil {
			errs = append(errs, errors.WithStack(l.udp.Close()))
		}
		return errs
	})
}

func (l *Listener) Accept() (Conn, error) {
	for {
		seg := l.pool.Get().(segment)

		seg.full()
		n, addr, err := l.udp.ReadFromUDPAddrPort(*seg)
		if err != nil {
			return nil, l.close(err)
		}
		seg.data(n)
		// todo: 校验数据包

		l.connsMu.RLock()
		a, has := l.conns[addr]
		l.connsMu.RUnlock()
		if !has {
			a = newAcceptConn(l, addr)
			l.connsMu.Lock()
			l.conns[addr] = a
			l.connsMu.Unlock()
		}
		a.put(seg)
		if !has {
			return a, nil
		}
	}
}
func (l *Listener) Addr() netip.AddrPort { return netip.MustParseAddrPort(l.udp.LocalAddr().String()) }
func (l *Listener) Close() error         { return l.close(nil) }

func (l *Listener) put(seg segment) {
	if seg == nil {
		return
	}
	l.pool.Put(seg)
}
func (l *Listener) del(raddr netip.AddrPort) {
	// tcp有ISN进行辅助判断, udp只能等待一段时间
	time.AfterFunc(time.Second, func() {
		l.connsMu.Lock()
		defer l.connsMu.Unlock()
		delete(l.conns, raddr)
	})
}
