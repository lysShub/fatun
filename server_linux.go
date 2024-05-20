//go:build linux
// +build linux

package fatun

import (
	"context"
	"net"
	"net/netip"
	"slices"
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

type sender struct {
	conn *eth.ETHConn
	to   net.HardwareAddr
}

func NewDefaultSender(laddr netip.AddrPort) (Sender, error) {
	ifi, err := ifaceByAddr(laddr.Addr())
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

	var s = &sender{to: to}
	s.conn, err = eth.Listen("eth:ip4", ifi)
	if err != nil {
		return nil, err
	}

	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(bpfFilterProtoAndLocalPorts(laddr.Port())); err != nil {
		return nil, s.close(errors.WithStack(err))
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
		return nil, s.close(errors.WithStack(err))
	}
	if e != nil {
		return nil, s.close(errors.WithStack(e))
	}

	return s, nil
}

func (s *sender) Recv(_ context.Context, ip *packet.Packet) error {
	n, _, err := s.conn.ReadFromHW(ip.Bytes())
	if err != nil {
		return err
	}
	ip.SetData(n)
	return nil
}

func (s *sender) Send(_ context.Context, ip *packet.Packet) error {
	_, err := s.conn.WriteToHW(ip.Bytes(), s.to)
	return err
}

func (s *sender) close(cause error) error {
	if s.conn != nil {
		if err := s.conn.Close(); err != nil && cause == nil {
			cause = err
		}
	}
	return cause
}
func (s *sender) Close() error { return s.close(nil) }

func bpfFilterProtoAndLocalPorts(skipPorts ...uint16) []bpf.Instruction {
	slices.Sort(skipPorts)
	skipPorts = slices.Compact(skipPorts)
	start := 0
	for i, e := range skipPorts {
		if e <= 1024 {
			start = i
		} else {
			break
		}
	}
	skipPorts = skipPorts[start+1:]

	const IPv4ProtocolOffset = 9
	var ins = []bpf.Instruction{
		// filter tcp/udp
		bpf.LoadAbsolute{Off: IPv4ProtocolOffset, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: unix.IPPROTO_TCP, SkipTrue: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: unix.IPPROTO_UDP, SkipTrue: 1},
		bpf.RetConstant{Val: 0},

		// store IPv4HdrLen regX
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 4, SkipTrue: 1},
		bpf.LoadMemShift{Off: 0},

		// skip port range (0, 1024]
		bpf.LoadIndirect{Off: header.TCPDstPortOffset, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpGreaterThan, Val: 1024, SkipTrue: 1},
		bpf.RetConstant{Val: 0},
	}
	for _, e := range skipPorts {
		ins = append(ins,
			bpf.LoadIndirect{Off: header.TCPDstPortOffset, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(e), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	return append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
}
