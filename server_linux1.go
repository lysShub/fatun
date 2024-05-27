//go:build linux
// +build linux

package fatun

import (
	"math/rand"
	"net"
	"net/netip"
	"slices"
	"sync/atomic"
	"unsafe"

	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/eth"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
	"github.com/lysShub/netkit/route"
	"github.com/mdlayher/arp"
	"github.com/pkg/errors"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type ethSender struct {
	conn *eth.ETHConn
	to   net.HardwareAddr
	id   atomic.Uint32 // ip id

	pcap *pcap.Pcap
}

// NewETHSender 即使关闭eth offload, 也会读取到许多超过mtu的数据包
func NewETHSender(laddr netip.AddrPort) ([]Sender, error) {
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

	var s = &ethSender{to: to}

	{
		s.pcap, err = pcap.File("client-segments.pcap")
		if err != nil {
			return nil, err
		}
	}

	s.id.Store(rand.Uint32())
	s.conn, err = eth.Listen("eth:ip4", ifi)
	if err != nil {
		return nil, err
	}

	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(bpfFilterProtoAndLocalTCPPorts(laddr.Port())); err != nil {
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

	return []Sender{s}, nil
}

func (s *ethSender) Recv(ip *packet.Packet) error {
	n, _, err := s.conn.ReadFromETH(ip.Bytes())
	if err != nil {
		return err
	}
	ip.SetData(n)

	{
		err = s.pcap.WriteIP(ip.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *ethSender) Send(ip *packet.Packet) error {
	hdr := header.IPv4(ip.Bytes())
	if hdr.More() {
		return errorx.WrapTemp(errors.New("can't send MF ip packet"))
	}
	hdr.SetTotalLength(uint16(len(hdr)))
	hdr.SetID(uint16(s.id.Add(1)))
	hdr.SetChecksum(^hdr.CalculateChecksum())

	{
		err := s.pcap.WriteIP(ip.Bytes())
		if err != nil {
			return err
		}
	}

	_, err := s.conn.WriteToETH(hdr, s.to)
	return err
}

func (s *ethSender) close(cause error) error {
	if s.conn != nil {
		if err := s.conn.Close(); err != nil && cause == nil {
			cause = err
		}
	}
	return cause
}
func (s *ethSender) Close() error { return s.close(nil) }

// bpfFilterProtoAndLocalTCPPorts bpf filter,
//
// will drop packet, that not tcp/udp, or dst-port in rang (0,1024], or tcp-dst-port in skipPorts
func bpfFilterProtoAndLocalTCPPorts(skipTCPPorts ...uint16) []bpf.Instruction {
	slices.Sort(skipTCPPorts)
	skipTCPPorts = slices.Compact(skipTCPPorts)
	start := len(skipTCPPorts)
	for i, e := range skipTCPPorts {
		if e > 1024 {
			start = i
			break
		}
	}
	skipTCPPorts = skipTCPPorts[start:]

	const IPv4ProtocolOffset = 9
	var ins = []bpf.Instruction{
		// filter tcp/udp
		bpf.LoadAbsolute{Off: IPv4ProtocolOffset, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: unix.IPPROTO_TCP, SkipTrue: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: unix.IPPROTO_UDP, SkipTrue: 1},
		bpf.RetConstant{Val: 0},

		// store IPv4HdrLen regX
		bpf.LoadMemShift{Off: 0},

		// skip port range (0, 1024]
		bpf.LoadIndirect{Off: header.TCPDstPortOffset, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpGreaterThan, Val: 1024, SkipTrue: 1},
		bpf.RetConstant{Val: 0},
	}
	if len(skipTCPPorts) > 0 {
		ins = append(ins,
			// skipTCPPorts not filter for udp
			bpf.LoadAbsolute{Off: IPv4ProtocolOffset, Size: 1},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: unix.IPPROTO_TCP, SkipTrue: 1},
			bpf.RetConstant{Val: 0xffff},
		)

		for _, e := range skipTCPPorts {
			ins = append(ins,
				bpf.LoadIndirect{Off: header.TCPDstPortOffset, Size: 2},
				bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(e), SkipTrue: 1},
				bpf.RetConstant{Val: 0},
			)
		}
	}

	return append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
}
