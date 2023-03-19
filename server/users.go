package server

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"unsafe"
)

type user struct {
	proxyConn net.Conn
}

func (u *user) do() {

	var b = make([]byte, 1532)
	var n int
	var err error
	for {

		b = b[:cap(b)]
		if n, err = u.proxyConn.Read(b); err != nil {
			panic(err)
		} else {
			if n < 20 {
				fmt.Println("非法数据包")
			}

			const reserved = 12
			if b[20+reserved]&0b111 == 0 { // Reversed = 0
				lport := *(*uint16)(unsafe.Pointer(&b[0]))
				if conns[lport] != nil && conns[lport].state == work {

					_, err = conns[lport].proxyConn.Write(b[:n])
					if err != nil {
						panic("")
					}
				} else {
					fmt.Println("未注册就发数据")
				}
			} else {
				var raddr netip.AddrPort

				var lport uint16
				if lport, err = registerConn(u.proxyConn, raddr); err != nil {
					panic(err)
				}

				b = binary.LittleEndian.AppendUint16(b[:n], lport)
				_, err = u.proxyConn.Write(b)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}
