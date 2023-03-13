package client

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/lysShub/go-divert"

	"golang.org/x/net/ipv4"
)

// TODO: 先不管DNS
type Client struct {
	m *sync.Mutex

	// client
	dialEvent chan dialEvent

	blockTime time.Duration

	serverIP net.IP

	setMapHandle divert.Handle
	setedCh      chan [6]byte

	// server
	redirectMap map[[6]byte]struct{}
}

type dialEvent struct {
	pid          int
	name         string
	laddr, raddr netip.AddrPort
}

func NewProxy(proxyServerIP net.IP) *Client {
	return nil
}

func (p *Client) do() {
	go p.listen()

	for event := range p.dialEvent {
		go func(e dialEvent) {
			if !p.block(e) {
				return
			}

			// proxy this conn
			err := p.proxy(e)
			if err != nil {
				fmt.Println(err)
			}
		}(event)
	}
}

func (p *Client) listen() {
	h, err := divert.Open("outbound and !loopback and tcp", divert.LAYER_FLOW, 111, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		panic(err)
	}

	var b []byte
	var addr divert.Address
	for {
		if _, addr, err = h.Recv(b); err != nil {
			panic(err)
		} else {
			if addr.Header.Event != divert.EVENT_FLOW_ESTABLISHED {
				continue
			}

			f := addr.Flow()

			name, err := getProcessName(int(f.ProcessId))
			if err != nil {
				fmt.Println(err)
				continue
			}

			select {
			case p.dialEvent <- dialEvent{
				int(f.ProcessId),
				name,
				f.LocalAddr(), f.RemoteAddr(),
			}:
			default:
			}

		}
	}
}

func (p *Client) block(e dialEvent) (block bool) {
	var f = fmt.Sprintf("!loopback and tcp and inbound and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d", e.laddr.Addr(), e.laddr.Port(), e.raddr.Addr(), e.raddr.Port())

	h, err := divert.Open(f, divert.LAYER_NETWORK, 91, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		panic(err)
	}

	block = true
	go func() {
		for {
			if n, _, err := h.Recv(nil); err != nil {
				if !strings.Contains(err.Error(), "close") {
					panic(err)
				}
			} else if n > 0 {
				block = false
				return
			}
		}
	}()

	var tt time.Duration
	for tt < p.blockTime {
		time.Sleep(time.Millisecond)
		tt = tt + time.Millisecond

		if !block {
			break
		}
	}
	h.Close()

	return block
}

// proxy add new proxy
func (p *Client) proxy(e dialEvent) error {
	if err := p.setMap(e); err != nil {
		return err
	}

	// open
	var f = fmt.Sprintf("!loopback and tcp and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d", e.laddr.Addr(), e.laddr.Port(), e.raddr.Addr(), e.raddr.Port())

	h, err := divert.Open(f, divert.LAYER_NETWORK, 51, divert.FLAG_READ_ONLY|divert.FLAG_WRITE_ONLY)
	if err != nil {
		panic(err)
	}

	// TODO: 先忽略, 因为有SYN RESEND
	// 发送syn

	// handle
	var b = make([]byte, 1536)
	var n int
	var addr divert.Address
	for {
		b = b[:1536]
		if n, addr, err = h.Recv(b); err != nil {
			panic(err)
		} else {
			b = b[:n]
			if addr.Header.Outbound() {
				// 修改dstIP为serverIP
				ipp(b).SetDstIP(p.serverIP)

				if _, err = h.Send(b, &addr); err != nil {
					panic(err)
				}
			} else {
				// 修改srcIP为remoteIP
				ipp(b).SetSrcIP(e.raddr.Addr())

				if _, err = h.Send(b, &addr); err != nil {
					panic(err)
				}
			}
		}
	}
}

type ipp []byte

func (i ipp) SetDstIP(ip net.IP) error {
	var ih = &ipv4.Header{}
	if err := ih.Parse(i); err != nil {
		return err
	}

	ih.Dst = ip
	ih.Checksum = 0

	b, err := ih.Marshal()
	if err != nil {
		return err
	}
	ih.Checksum = int(checkSum(b))

	b, err = ih.Marshal()
	if err != nil {
		return err
	}
	copy(i[0:], b)
	return nil
}

func (i ipp) SetSrcIP(ip netip.Addr) error {
	var ih = &ipv4.Header{}
	if err := ih.Parse(i); err != nil {
		return err
	}

	sip := ip.As4()
	ih.Src = sip[:]
	ih.Checksum = 0

	b, err := ih.Marshal()
	if err != nil {
		return err
	}
	ih.Checksum = int(checkSum(b))

	b, err = ih.Marshal()
	if err != nil {
		return err
	}
	copy(i[0:], b)
	return nil
}

func checkSum(d []byte) uint16 {
	var S uint32
	l := len(d)
	if l&0b1 == 1 {
		for i := 0; i < l-1; {
			S = S + uint32(d[i])<<8 + uint32(d[i+1])
			if S>>16 > 0 {
				S = S&0xffff + 1
			}
			i = i + 2
		}
		S = S + uint32(d[l-1])<<8
	} else {
		for i := 0; i < l; {
			S = S + uint32(d[i])<<8 + uint32(d[i+1])
			if S>>16 > 0 {
				S = S&0xffff + 1
			}
			i = i + 2
		}
	}

	return uint16(65535) - uint16(S)
}
