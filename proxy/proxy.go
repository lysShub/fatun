package proxy

import (
	"context"
	"fmt"
	"itun/pack"
	"net"
	"sync"
	"unsafe"

	"github.com/lysShub/go-divert"
)

/*

	代理

*/

const listenPri = 27
const injectPri = 11
const capturPri = 23

type Proxy struct {
	m *sync.Mutex

	ifi     int
	localIP [4]byte

	ctx    context.Context
	cancel context.CancelCauseFunc

	proxyConn net.Conn

	// recv
	ipHdr *pack.IPHdr

	// send
	listenHdl divert.Handle
	injectHdl divert.Handle
	states    [65536]state
	proxys    [65536]*captur
}

func NewProxy(ctx context.Context, proxConn *net.UDPConn) (*Proxy, error) {
	var p = &Proxy{}
	var err error

	p.injectHdl, err = divert.Open("false", divert.LAYER_NETWORK, injectPri, divert.FLAG_WRITE_ONLY)
	if err != nil {
		return nil, err
	}

	p.ctx, p.cancel = context.WithCancelCause(ctx)
	return p, nil
}

func (p *Proxy) shutdown(err error) bool {
	if err != nil {
		p.cancel(err)
		// TODO: CLOSE ALL
		return true
	}
	return false
}

func (p *Proxy) listen() {
	var f = fmt.Sprintf("ip and !loopback and outbound and (tcp.Syn or tcp.Fin)")

	var err error
	p.listenHdl, err = divert.Open(f, divert.LAYER_NETWORK, listenPri, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if p.shutdown(err) {
		return
	}

	var b = make([]byte, 128)
	var n int
	var lport uint16
	for {
		n, _, err = p.listenHdl.Recv(b)
		if p.shutdown(err) {
			return
		} else {
			if n >= 40 {

				iphl := (b[0] >> 4) * 5

				lport = *(*uint16)(unsafe.Pointer(&b[iphl]))

				if b[iphl+13]&0b10 == 0b01 { // syn
					if p.states[lport] == once {
						p.states[lport] = clog

						// proxy

					} else {
						p.states[lport] = once
					}
				} else if b[iphl+13]&0b1 == 0b1 { // fin
					p.states[lport] = idle
				}
			}
		}
	}
}
