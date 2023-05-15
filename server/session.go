package server

import (
	"encoding/binary"
	"fmt"
	"itun/pack"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/net/ipv4"
)

type session struct {
	working          atomic.Bool
	oldInc, inc, cyc uint8
	proto            uint8
	srcPort          uint16
	locPort          uint16
	dstAddr          netip.AddrPort
	capture          *ipv4.PacketConn
}

func (s *session) Capture(sendto net.Conn, locIP net.IP) {
	s.working.Store(true)
	defer func() { s.working.Store(false) }()

	var err error
	s.capture, err = s.getCapture(locIP)
	if err != nil {
		s.close(err)
	}

	var b = make([]byte, 1532)
	var dstPort *uint16 = (*uint16)(unsafe.Pointer(&b[2]))
	var relDstPortBig uint16 = toBig(s.dstAddr.Port())
	var (
		n  int
		cm *ipv4.ControlMessage
	)

	for s.working.Load() {
		b = b[:cap(b)]
		n, cm, _, err = s.capture.ReadFrom(b)
		if err != nil {
			s.close(err)
		}
		*dstPort = relDstPortBig

		srcAddr, _ := netip.AddrFromSlice(cm.Src)

		_, err = sendto.Write(pack.Packe(b[:n], s.proto, srcAddr))
		if err != nil {
			s.close(err)
		}

		s.inc++
	}
}

func (s *session) getCapture(locIP net.IP) (*ipv4.PacketConn, error) {
	conn, err := net.ListenIP("ip4:"+strconv.Itoa(int(s.proto)), &net.IPAddr{IP: locIP})
	if err != nil {
		return nil, err
	}

	c := ipv4.NewPacketConn(conn)
	err = c.SetControlMessage(ipv4.FlagSrc, true)
	if err != nil {
		return nil, err
	}

	var filter = []bpf.Instruction{
		// only IPv4
		//   TODO: bpf.LoadExtension{Num: bpf.ExtProto},
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.LoadConstant{Dst: bpf.RegX, Val: 4},
		bpf.ALUOpX{Op: bpf.ALUOpShiftRight},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 4, SkipTrue: 1},
		bpf.RetConstant{Val: 0x00},

		// check L4 protocol
		bpf.LoadAbsolute{Off: 9, Size: 1},
		bpf.JumpIf{
			Cond:     bpf.JumpEqual,
			Val:      uint32(s.proto),
			SkipTrue: 1,
		},
		bpf.RetConstant{Val: 0x00},

		// check srcIP
		bpf.LoadAbsolute{Off: 0 + 12, Size: 4},
		bpf.JumpIf{
			Cond: bpf.JumpEqual,
			Val:  binary.BigEndian.Uint32(s.dstAddr.Addr().AsSlice()),
		},
		bpf.RetConstant{Val: 0x00},

		// check srcPort
		bpf.LoadMemShift{Off: 0}, // load HdrLen to regX
		bpf.LoadIndirect{Off: 0, Size: 2},
		bpf.JumpIf{
			Cond: bpf.JumpEqual,
			Val:  uint32(s.dstAddr.Port()),
		},
		bpf.RetConstant{Val: 0x00},

		// check dstPort
		bpf.LoadIndirect{Off: 2, Size: 2},
		bpf.JumpIf{
			Cond: bpf.JumpEqual,
			Val:  uint32(s.locPort),
		},
		bpf.RetConstant{Val: 0x00},

		// capture this packet
		bpf.RetConstant{Val: 0xffff},
	}
	rawFilter, err := bpf.Assemble(filter)
	if err != nil {
		return nil, err
	}
	if err = c.SetBPF(rawFilter); err != nil {
		return nil, err
	}

	return c, nil
}

func (s *session) idle() bool {
	return !s.working.Load()
}

func (s *session) check() (kill bool) {
	if s.idle() {
		return false
	} else {
		if s.oldInc == s.inc {
			s.cyc++
			if s.cyc > 3 {
				return true
			}
		} else {
			s.oldInc = s.inc
			s.cyc = 0
		}
		return false
	}
}

func (s *session) close(err error) {
	if s.working.CompareAndSwap(true, false) {
		fmt.Println("close session:", err)

		if s.capture != nil {
			s.capture.Close()
		}
		*s = session{}
	}
}
