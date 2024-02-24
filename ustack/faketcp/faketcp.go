package faketcp

import (
	"sync/atomic"

	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// record tcp seq/ack, not care handshake/clode, etc.
// todo: more reasonable wnd
type FakeTCP struct {
	lport, rport uint16
	seq          atomic.Uint32
	ack          atomic.Uint32

	// todo: use alloce tcp header bytes
	// header []byte

	checksum bool
}

// NewFakeTCP set fake tcp header
func NewFakeTCP(locPort, remPort uint16, initSeq, initAck uint32, checksum bool) *FakeTCP {
	f := &FakeTCP{
		lport: locPort,
		rport: remPort,
	}
	f.seq.Store(initSeq)
	f.ack.Store(initAck)
	return f
}

// SendAttach input tcp payload, attach tcp header, and return
// tcp packet.
func (f *FakeTCP) SendAttach(p *relraw.Packet) {
	var b = make(header.TCP, header.TCPMinimumSize)
	b.Encode(&header.TCPFields{
		SrcPort:    f.lport,
		DstPort:    f.rport,
		SeqNum:     f.seq.Load(),
		AckNum:     f.ack.Load(),
		DataOffset: header.TCPMinimumSize,
		Flags:      header.TCPFlagPsh | header.TCPFlagAck,
		WindowSize: 0xff32, // todo: rand
		Checksum:   0,
	})
	if f.checksum {
		sum := checksum.Checksum(p.Data(), 0)
		b.SetChecksum(^checksum.Checksum(b, sum))
	}

	f.seq.Add(uint32(p.Len()))
	p.Attach(b)
}

// RecvStrip input a tcp packet, update ack, and return
// tcp payload.
func (f *FakeTCP) RecvStrip(tcp *relraw.Packet) {
	tcphdr := header.TCP(tcp.Data())

	new := tcphdr.SequenceNumber() + uint32(len(tcphdr.Payload()))
	old := f.ack.Load()

	if new > old {
		f.ack.Store(new)
	}

	// remove tcp header
	tcp.SetHead(tcp.Head() + int(tcphdr.DataOffset()))
}
