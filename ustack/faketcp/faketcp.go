package faketcp

import (
	"sync/atomic"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
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

	pseudoSum1 *uint16
}

// NewFakeTCP set fake tcp header
func NewFakeTCP(locPort, remPort uint16, initSeq, initAck uint32, pseudoSum1 *uint16) *FakeTCP {
	f := &FakeTCP{
		lport:      locPort,
		rport:      remPort,
		pseudoSum1: pseudoSum1,
	}
	f.seq.Store(initSeq)
	f.ack.Store(initAck)
	return f
}

// SendAttach input tcp payload, attach tcp header, and return
// tcp packet.
func (f *FakeTCP) SendAttach(p *relraw.Packet) {
	var hdr = make(header.TCP, header.TCPMinimumSize)
	hdr.Encode(&header.TCPFields{
		SrcPort:       f.lport,
		DstPort:       f.rport,
		SeqNum:        f.seq.Load(),
		AckNum:        f.ack.Load(),
		DataOffset:    header.TCPMinimumSize,
		Flags:         header.TCPFlagPsh | header.TCPFlagAck,
		WindowSize:    0xff32, // todo: mock
		Checksum:      0,
		UrgentPointer: 0,
	})

	f.seq.Add(uint32(p.Len()))
	p.Attach(hdr)

	if f.pseudoSum1 != nil {
		tcp := header.TCP(p.Data())
		psum := checksum.Combine(*f.pseudoSum1, uint16(len(tcp)))
		sum := checksum.Checksum(tcp, psum)
		tcp.SetChecksum(^sum)
	}

	if debug.Debug() {
		test.ValidTCP(test.T(), p.Data(), *f.pseudoSum1)
	}
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
