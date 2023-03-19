package chans

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"unsafe"
)

const chunk = 512

type Chans struct {
	net.TCPConn
}

func (c *Chans) NewChannel(newConn net.TCPConn, laddr, sAddr netip.AddrPort) error {

	if (sAddr.Addr().Is6() && !sAddr.Addr().Is4In6()) ||
		(sAddr.Addr().Is6() && sAddr.Addr().Is4In6()) {
		return errors.New("unsupported IPv6")
	}

	go func(conn net.TCPConn, dstIP [4]byte) {
		var b = make([]byte, chunk, chunk+6)
		*(*[4]byte)(unsafe.Pointer(&b[len(b)])) = dstIP

		// dstIP:len
		for {
			if n, err := newConn.Read(b); err != nil {
				fmt.Println("client read err: ", conn.LocalAddr(), &net.TCPAddr{IP: []byte(dstIP[:]), Port: 0}, err)
				return
			} else {
				*(*uint16)(unsafe.Pointer(&b[len(b)+4])) = uint16(n)

				if _, err := c.TCPConn.Write(b[:cap(b)]); err != nil {
					fmt.Println("proxy write err: ", err)
					return
				}
			}
		}
	}(newConn, sAddr.Addr().As4())

	return nil
}
