package server

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"unsafe"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// 不会更改端口
type Server struct {
	m *sync.RWMutex

	// key: src 端口
	// value: dst IP, src IP
	dstMapping map[uint16][4]byte
	srcMapping map[uint16][4]byte
	livMapping map[uint16]bool
}

func NewServer() (*Server, error) {
	var s = &Server{
		m:          &sync.RWMutex{},
		dstMapping: map[uint16][4]byte{},
		srcMapping: map[uint16][4]byte{},
		livMapping: map[uint16]bool{},
	}

	rc, err := net.ListenIP("ip4:1", nil)
	if err != nil {
		return nil, err
	}
	icmpConn, err := ipv4.NewRawConn(rc)
	if err != nil {
		return nil, err
	}
	go s.listenIcmp(icmpConn)

	return s, nil
}

func (s *Server) listenIcmp(conn *ipv4.RawConn) {
	var da = make([]byte, 512)
	var port uint16
	var dstIP, srcIP [4]byte

	var n int
	var addr *net.IPAddr
	var err error
	for {
		n, addr, err = conn.ReadFromIP(da)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			if nip, ok := netip.AddrFromSlice(addr.IP); !ok {
				fmt.Println("invalid ip addr ", addr.String())
			} else if !nip.Is4() {
				fmt.Println("only support IPv4, get", addr.String())
			} else {
				srcIP = nip.As4()
			}

			if n > 28 {
				ping, err := icmp.ParseMessage(1, da[:n])
				if err != nil {
					fmt.Println(err)
					continue
				}
				if v, ok := ping.Body.(*icmp.RawBody); ok && len(v.Data) == 32 {
					port = *(*uint16)(unsafe.Pointer(&v.Data[0]))
					dstIP = [4]byte{v.Data[2], v.Data[3], v.Data[4], v.Data[5]}

					// set map
					s.m.Lock()
					s.dstMapping[port] = dstIP
					s.srcMapping[port] = srcIP
					s.m.Unlock()
				}
			}
		}
	}
}

func (s *Server) listenIP(conn *ipv4.RawConn) {
	var da = make([]byte, 1536)

	var getIP, srcIP, dstIP [4]byte
var has bool

	var n int
	var addr *net.IPAddr
	var err error
	for {
		n, addr, err = conn.ReadFromIP(da)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			if nip, ok := netip.AddrFromSlice(addr.IP); !ok {
				fmt.Println("invalid ip addr ", addr.String())
			} else if !nip.Is4() {
				fmt.Println("only support IPv4, get", addr.String())
			} else {
				getIP = nip.As4()
			}

			if n > 40 {

				// 获取srcIP, 如果存在(serverIPs), 说明来自server, 不存在则来自client
				//  来自client: 记录(sip:cip), 并

				s.m.RLock()
				if  s.srcMapping[] {
					
				}

				var has bool
				s.m.RLock()
				_, has = s.srcMapping[srcIP]
				s.m.RUnlock()

				if has { // 来自 client

					iph, err := ipv4.ParseHeader(da)
					if err != nil {
						fmt.Println(err)
						continue
					}
					port := *(*uint16)(unsafe.Pointer(&da[iph.Len]))

					s.m.RLock()
					// dstIP, has = s.dstMapping[port][]
					s.m.Unlock()

				} else {

				}
			}
		}
	}
}
