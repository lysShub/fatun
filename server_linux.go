//go:build linux
// +build linux

package fatun

import (
	"net"
	"net/netip"
	"slices"
	"syscall"
	"unsafe"

	"github.com/lysShub/netkit/eth"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/mdlayher/arp"
	"github.com/pkg/errors"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func NewDefaultSender(laddr netip.AddrPort) (Sender, error) {
	s, err := NewETHSender(laddr.Addr())
	if err != nil {
		return nil, err
	}
	if err = s.SkipPorts(
		[]uint16{22},
		[]uint16{laddr.Port()}, // todo: current work on udp
	); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

type EthSender struct {
	conn *eth.ETHConn
	to   net.HardwareAddr
}

var _ Sender = (*EthSender)(nil)

func NewETHSender(laddr netip.Addr) (*EthSender, error) {
	ifi, err := ifaceByAddr(laddr)
	if err != nil {
		return nil, err
	}

	var to net.HardwareAddr
	if c, err := arp.Dial(ifi); err != nil {
		return nil, errors.WithStack(err)
	} else {
		var gateway netip.Addr
		if rows, err := route.GetTable(); err != nil {
			return nil, err
		} else {
			for _, e := range rows {
				if e.Interface == uint32(ifi.Index) {
					gateway = e.Next
					break
				}
			}
		}
		if !gateway.IsValid() {
			return nil, errors.Errorf("interface %s not network connect", ifi.Name)
		}
		if to, err = c.Resolve(gateway); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	var s = &EthSender{to: to}

	s.conn, err = eth.Listen("eth:ip4", ifi)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *EthSender) SkipPorts(tcp, udp []uint16) error {
	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(bpfFilterProtoAndSkipLocalTCPPorts(tcp, udp)); err != nil {
		return s.close(errors.WithStack(err))
	} else {
		prog = &unix.SockFprog{
			Len:    uint16(len(rawIns)),
			Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
		}
	}

	var e error
	if err := s.conn.SyscallConn().Control(func(fd uintptr) {
		e = unix.SetsockoptSockFprog(int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog)
	}); err != nil {
		return s.close(errors.WithStack(err))
	}
	if e != nil {
		return s.close(errors.WithStack(e))
	}
	return nil
}

func (s *EthSender) Recv(ip *packet.Packet) error {
	n, _, err := s.conn.ReadFromETH(ip.Bytes())
	if err != nil {
		return err
	}
	ip.SetData(n)

	return nil
}

func (s *EthSender) Send(ip *packet.Packet) error {
	_, err := s.conn.WriteToETH(ip.Bytes(), s.to)
	return err
}

func (s *EthSender) close(cause error) error {
	if s.conn != nil {
		if err := s.conn.Close(); err != nil && cause == nil {
			cause = err
		}
	}
	return cause
}
func (s *EthSender) Close() error { return s.close(nil) }

// bpfFilterProtoAndSkipLocalTCPPorts bpf filter,
//
// will drop packet, that the protocol dst port in skip-ports
func bpfFilterProtoAndSkipLocalTCPPorts(tcpSkipPorts, udpSkipPorts []uint16) (ins []bpf.Instruction) {
	ins = append(ins,
		bpfSkipPorts(syscall.IPPROTO_TCP, tcpSkipPorts)...,
	)
	ins = append(ins,
		bpfSkipPorts(syscall.IPPROTO_UDP, udpSkipPorts)...,
	)

	return append(ins,
		bpf.RetConstant{Val: 0},
	)
}

func bpfSkipPorts(proto uint8, ports []uint16) []bpf.Instruction {
	slices.Sort(ports)
	ports = slices.Compact(ports)

	const IPv4ProtocolOffset = 9

	var skipIns = []bpf.Instruction{
		// store IPv4HdrLen regX
		bpf.LoadMemShift{Off: 0},
	}

	const DstPortOffset = header.TCPDstPortOffset
	for _, e := range ports {
		skipIns = append(skipIns,

			bpf.LoadIndirect{Off: DstPortOffset, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(e), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}
	skipIns = append(skipIns,
		bpf.RetConstant{Val: 0xffff},
	)

	var ins = []bpf.Instruction{
		bpf.LoadAbsolute{Off: IPv4ProtocolOffset, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(proto), SkipTrue: uint8(len(skipIns))},
	}
	ins = append(ins, skipIns...)

	return ins
}
