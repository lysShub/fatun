package client

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
	"unsafe"

	"github.com/lysShub/go-divert"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func getProcessName(pid int) (string, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return "", err
	}

	n, err := p.Name()
	if err != nil {
		return "", err
	}
	return n, nil
}

func getLocalIP() net.IP {
	if conn, err := net.Dial("udp", "114.114.114.114:80"); err != nil {
		return nil
	} else {
		ip := net.ParseIP(strings.Split(conn.LocalAddr().String(), ":")[0])
		conn.Close()
		return ip.To4()
	}
}

// remotePort:remoteIP
func (p *Client) setMap(e dialEvent) (err error) {
	p.m.Lock()
	if p.setMapHandle == 0 {
		f := fmt.Sprintf("!loopback and icmp and remoteAddr=%s", p.serverIP)
		p.setMapHandle, err = divert.Open(f, divert.LAYER_NETWORK, 111, 0)
		if err != nil {
			return err
		}
		go func() {
			var da = make([]byte, 256)
			for {
				if n, _, err := p.setMapHandle.Recv(da); err != nil {
					p.setMapHandle.Close()
					p.setMapHandle = 0
				} else if n > 28 {
					if cap(p.setedCh) == len(p.setedCh) {
						<-p.setedCh
					}
					select {
					case p.setedCh <- [6]byte{da[28], da[29], da[30], da[31], da[32], da[33]}:
					default:
					}
				}
			}
		}()
	}
	p.m.Unlock()

	var info [6]byte
	port := e.raddr.Port()
	*(*uint16)(unsafe.Pointer(&info[0])) = port
	*(*[4]byte)(unsafe.Pointer(&info[2])) = e.raddr.Addr().As4()

	var payload = make([]byte, 32)
	rand.Read(payload)
	copy(payload[0:], info[:])
	var b = &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID: e.pid & 0xffff, Seq: 1,
			Data: payload,
		},
	}
	icmpData, err := b.Marshal(nil)
	if err != nil {
		return err
	}

	var ih = &ipv4.Header{
		Version:  4,
		Len:      20,
		TOS:      0,
		TotalLen: 20 + len(icmpData),
		ID:       e.pid,
		Flags:    ipv4.DontFragment,
		FragOff:  0,
		TTL:      128,
		Protocol: 1,
		Checksum: 0,
		Src:      toIP(e.laddr.Addr()),
		Dst:      p.serverIP,
	}
	ipHeader, err := ih.Marshal()
	if err != nil {
		return err
	}
	ih.Checksum = int(checkSum(ipHeader))
	ipHeader, err = ih.Marshal()
	if err != nil {
		return err
	}

	var data = append(ipHeader, icmpData...)

	addr := &divert.Address{}
	addr.Header.Layer = divert.LAYER_NETWORK
	addr.Header.Event = divert.EVENT_NETWORK_PACKET
	addr.Header.SetOutbound(true)

	var ok bool = false
	defer func() { ok = true }()
	for i := 0; i < 3; i++ {
		time.AfterFunc(time.Millisecond*100*time.Duration(i), func() {
			if !ok {
				if _, err = p.setMapHandle.Send(data, addr); err != nil {
					p.setedCh <- [6]byte{}
					return
				}
			}
		})
	}

	for {
		select {
		case i := <-p.setedCh:
			if i == info || err != nil {
				return err
			}
		case <-time.After(time.Millisecond * 300):
			if err == nil {
				return errors.New("server unreachable")
			}
		}
	}
}

func toIP(ip netip.Addr) net.IP {
	p := ip.As4()
	return net.IPv4(p[0], p[1], p[2], p[3])
}

func genTcpSYN(e dialEvent) ([]byte, error) {

	return nil, nil
}
