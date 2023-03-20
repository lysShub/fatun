package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	"github.com/lysShub/go-divert"
	"golang.org/x/net/ipv4"
)

type Proxy struct {
	acceptHdl divert.Handle
	ech       chan event

	syncs  [0xffff]uint8
	proxys [0xffff]proxyer
}

type event struct {
	laddr netip.AddrPort
	raddr netip.AddrPort
	syn   []byte
}

func NewProxy(proxyConn net.Conn) {}

func (p *Proxy) Do(ctx context.Context) error {

	for {
		event, err := p.accept()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(event)
	}

	return nil
}

func (p *Proxy) SetFilter(f string) error { return nil }

// accept
//
//	accept proxy event for default strategy
func (p *Proxy) accept() (e event, err error) {
	const f = "ip and !loopback and outbound and tcp.Syn"
	const tcpHdrLen = 20

	if p.acceptHdl == 0 {
		p.acceptHdl, err = divert.Open(f, divert.LAYER_NETWORK, 0, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
		if err != nil {
			return e, err
		}
	}

	var b = make([]byte, 128)
	var n int
	var lport, rport uint16
	var iphdr ipv4.Header
	for {
		n, _, err = p.acceptHdl.Recv(b)
		if err != nil {
			return e, err
		} else {

			err = iphdr.Parse(b[:n])
			if err != nil || n < iphdr.Len+tcpHdrLen {
				if err == nil {
					err = errors.New("")
				}
				return e, err
			} else {
				lport = *(*uint16)(unsafe.Pointer(&b[iphdr.Len]))
				rport = *(*uint16)(unsafe.Pointer(&b[iphdr.Len+2]))

				p.syncs[lport]++
				if p.syncs[lport] >= 1 {
					e.laddr = netip.AddrPortFrom(netip.AddrFrom4(to(iphdr.Src)), lport)
					e.raddr = netip.AddrPortFrom(netip.AddrFrom4(to(iphdr.Dst)), rport)
					e.syn = make([]byte, n-iphdr.Len)
					copy(e.syn, b[iphdr.Len:])
					return e, nil
				}
			}
		}
	}
}

func to(ip net.IP) [4]byte { return *(*[4]byte)(unsafe.Pointer(&ip[0])) }

func (p *Proxy) Close() error { return nil }
