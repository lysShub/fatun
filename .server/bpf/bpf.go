package bpf

import (
	"encoding/binary"
	"net/netip"

	"golang.org/x/net/bpf"
)

type Filter []Pcap

var None = []bpf.RawInstruction{}
var All = []bpf.RawInstruction{}

func init() {
	var f1 = []bpf.Instruction{bpf.RetConstant{Val: 0}}
	None, _ = bpf.Assemble(f1)
	var f2 = []bpf.Instruction{bpf.RetConstant{Val: 0xffff}}
	All, _ = bpf.Assemble(f2)
}

type raw = []bpf.RawInstruction

func (p Filter) Assemble() (raw, error) {
	var fs = []bpf.Instruction{}
	for _, pac := range p {
		fs = append(fs, pac.filter()...)
	}
	fs = append(fs, bpf.RetConstant{Val: 0})

	return bpf.Assemble(fs)
}

type Pcap struct {
	Proto   uint8
	LocPort uint16
	Src     netip.AddrPort
}

func (p Pcap) filter() []bpf.Instruction {
	const ethHdrLen = 14
	const next = 0xff

	var r = []bpf.Instruction{

		// only IPv4
		//   TODO: bpf.LoadExtension{Num: bpf.ExtProto},
		bpf.LoadAbsolute{Off: ethHdrLen, Size: 1},
		bpf.LoadConstant{Dst: bpf.RegX, Val: 4},
		bpf.ALUOpX{Op: bpf.ALUOpShiftRight},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 4, SkipTrue: next},

		// check L4 protocol
		bpf.LoadAbsolute{Off: ethHdrLen + 9, Size: 1},
		bpf.JumpIf{
			Cond:     bpf.JumpNotEqual,
			Val:      uint32(p.Proto),
			SkipTrue: next,
		},

		// check srcIP
		bpf.LoadAbsolute{Off: ethHdrLen + 12, Size: 4},
		bpf.JumpIf{
			Cond:     bpf.JumpNotEqual,
			Val:      binary.BigEndian.Uint32(p.Src.Addr().AsSlice()),
			SkipTrue: next,
		},

		// check srcPort
		bpf.LoadMemShift{Off: ethHdrLen},          // 把HdrLen加载到X
		bpf.LoadIndirect{Off: ethHdrLen, Size: 2}, // 把srcPort加载到A
		bpf.JumpIf{
			Cond:     bpf.JumpNotEqual,
			Val:      uint32(p.Src.Port()),
			SkipTrue: next,
		},

		// check dstPort
		bpf.LoadIndirect{Off: ethHdrLen + 2, Size: 2}, // 把dstPort加载到A
		bpf.JumpIf{
			Cond:     bpf.JumpNotEqual,
			Val:      uint32(p.LocPort),
			SkipTrue: next,
		},

		// capture this packet
		bpf.RetConstant{Val: 0xffff},

		// next, for next pacp filter
	}

	// reset next
	n := len(r)
	for i, f := range r {
		if jf, ok := f.(bpf.JumpIf); ok {
			if jf.SkipTrue == next && jf.SkipFalse == 0 {
				r[i] = bpf.JumpIf{
					Cond:     jf.Cond,
					Val:      jf.Val,
					SkipTrue: uint8(n - i - 1),
				}
			} else if jf.SkipFalse == next && jf.SkipTrue == 0 {
				r[i] = bpf.JumpIf{
					Cond:      jf.Cond,
					Val:       jf.Val,
					SkipFalse: uint8(n - i - 1),
				}
			} else {
				panic("invalid bpf.JumpIf")
			}
		}
	}
	return r
}
