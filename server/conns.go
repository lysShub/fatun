package server

import (
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/net/ipv4"
)

var conns [0xffff]*conn

type conn struct {
	socket    syscall.Handle // hold port
	proxyConn net.Conn

	ipConn *ipv4.PacketConn

	proxyerPort uint16
	state       state
}

type state uint8

const (
	_ state = iota
	idle
	prepare
	work
	unknown
)

func registerConn(proxyConn net.Conn, raddr netip.AddrPort) (lport uint16, err error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		return 0, err
	}
	var laddr = &syscall.SockaddrInet4{}
	if err = syscall.Bind(fd, laddr); err != nil {
		return 0, err
	}

	var c = &conn{
		socket:      fd,
		proxyerPort: uint16(laddr.Port),

		proxyConn: proxyConn,
		state:     prepare,
	}

	pconn, err := net.ListenPacket("ip4:tcp", "")
	if err != nil {
		return 0, err
	}
	c.ipConn = ipv4.NewPacketConn(pconn)

	tcpFilter, err := getFilter(c.proxyerPort, raddr.Port(), raddr.Addr().As4())
	if err != nil {
		return 0, err
	}
	if err = c.ipConn.SetBPF(tcpFilter); err != nil {
		return 0, err
	}

	conns[c.proxyerPort] = c
	return c.proxyerPort, nil
}

func (c *conn) do() {
	var b = make([]byte, 1532)
	var n int
	var err error
	for {

		n, _, _, err = c.ipConn.ReadFrom(b)
		if err != nil {
			fmt.Println(err)
			return
		}

		_, err = c.proxyConn.Write(b[:n])
		if err != nil {
			fmt.Println(err)
			return
		}
	}

}

func getFilter(lport, rport uint16, raddr [4]byte) (f []bpf.RawInstruction, err error) {

	var tcpFilter = []bpf.Instruction{

		bpf.LoadAbsolute{Off: 12, Size: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: *(*uint32)(unsafe.Pointer(&raddr)), SkipTrue: 1},
		bpf.RetConstant{Val: 0x0},

		bpf.LoadMemShift{Off: 0},
		bpf.LoadIndirect{0, 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(rport), SkipTrue: 1},
		bpf.RetConstant{Val: 0x0},

		bpf.LoadIndirect{0, 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(lport), SkipTrue: 1},
		bpf.RetConstant{Val: 0x0},

		bpf.RetConstant{Val: 0xffff},
	}

	return bpf.Assemble(tcpFilter)
}
