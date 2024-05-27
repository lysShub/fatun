//go:build linux
// +build linux

package fatun

import (
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/helper"
	"github.com/pkg/errors"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type IPSender struct {
	tcp   *net.IPConn
	udp   *net.IPConn
	istcp bool
}

func NewIPSender(laddr netip.AddrPort) (sneders []Sender, err error) {
	tcp, err := net.ListenIP("ip4:tcp", &net.IPAddr{IP: laddr.Addr().AsSlice()})
	defer func() {
		if err != nil {
			tcp.Close()
		}
	}()
	if err != nil {
		return nil, errors.WithStack(err)
	} else {
		var prog *unix.SockFprog
		if rawIns, err := bpf.Assemble(bpfFilterProtoAndLocalTCPPorts(laddr.Port())); err != nil {
			return nil, errors.WithStack(err)
		} else {
			prog = &unix.SockFprog{
				Len:    uint16(len(rawIns)),
				Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
			}
		}

		var e error
		if raw, err := tcp.SyscallConn(); err != nil {
			return nil, errors.WithStack(err)
		} else {
			if err := raw.Control(func(fd uintptr) {
				e = unix.SetsockoptSockFprog(int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog)
				if e != nil {
					return
				}
				e = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_HDRINCL, 1)
			}); err != nil {
				return nil, errors.WithStack(err)
			}
			if e != nil {
				return nil, errors.WithStack(err)
			}
		}
	}

	udp, err := net.ListenIP("ip4:tcp", &net.IPAddr{IP: laddr.Addr().AsSlice()})
	defer func() {
		if err != nil {
			udp.Close()
		}
	}()
	if err != nil {
		return nil, errors.WithStack(err)
	} else {
		var e error
		if raw, err := tcp.SyscallConn(); err != nil {
			return nil, errors.WithStack(err)
		} else {
			if err := raw.Control(func(fd uintptr) {
				e = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_HDRINCL, 1)
			}); err != nil {
				return nil, errors.WithStack(err)
			}
			if e != nil {
				return nil, errors.WithStack(err)
			}
		}
	}

	return []Sender{
		&IPSender{tcp: tcp, udp: udp, istcp: true},
		&IPSender{tcp: tcp, udp: udp},
	}, nil
}

func (s *IPSender) Recv(ip *packet.Packet) (err error) {
	var n int
	if s.istcp {
		n, err = s.tcp.Read(ip.Bytes())
	} else {
		n, err = s.udp.Read(ip.Bytes())
	}
	if err != nil {
		return errors.WithStack(err)
	}
	ip.SetData(n)

	// todo: unix.MSG_TRUNC
	if _, err := helper.IPCheck(ip.Bytes()); err != nil {
		return err
	}
	return nil
}

func (s *IPSender) Send(ip *packet.Packet) (err error) {
	var hdr = header.IPv4(ip.Bytes())
	var dst = hdr.DestinationAddress()
	switch proto := hdr.TransportProtocol(); proto {
	case header.TCPProtocolNumber:
		_, err = s.tcp.WriteToIP(ip.Bytes(), &net.IPAddr{IP: dst.AsSlice()})
	case header.UDPProtocolNumber:
		_, err = s.udp.WriteToIP(ip.Bytes(), &net.IPAddr{IP: dst.AsSlice()})
	default:
		panic(fmt.Sprintf("not support  protocol %d", proto))
	}
	return errors.WithStack(err)
}

func (s *IPSender) Close() (err error) {
	if s.tcp != nil {
		err = s.tcp.Close()
	}
	if s.udp != nil {
		err = s.udp.Close()
	}
	return err
}
