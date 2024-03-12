package faketcp

import (
	"math/rand"
	"sync/atomic"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// record tcp seq/ack, not care handshake/clode, etc.
// todo: more reasonable wnd
type FakeTCP struct {
	lport, rport uint16

	// todo: use alloce tcp header bytes
	// header []byte

	seq atomic.Uint32
	ack atomic.Uint32

	seqOverhead uint32

	pseudoSum1 *uint16
}

type options struct {
	initSeq       uint32
	initAck       uint32
	seqOverhead   uint32
	pseudoSum1    uint16
	pseudoSum1Set bool
}

func defaultOpts() *options {
	return &options{
		initSeq: rand.Uint32(),
		initAck: rand.Uint32(),
	}
}

func InitSeqAck(seq, ack uint32) func(opt *options) {
	return func(opt *options) {
		opt.initSeq = seq
		opt.initAck = ack
	}
}

// SeqOverhead additional seq delta each tcp packet
func SeqOverhead(delta uint32) func(opt *options) {
	return func(opt *options) { opt.seqOverhead = delta }
}

// PseudoSum1 if set, will calc tcp header checksum
func PseudoSum1(s uint16) func(opt *options) {
	return func(opt *options) {
		opt.pseudoSum1 = s
		opt.pseudoSum1Set = true
	}
}

// NewFakeTCP set fake tcp header
func NewFakeTCP(locPort, remPort uint16, opts ...func(opt *options)) *FakeTCP {
	opt := defaultOpts()
	for _, e := range opts {
		e(opt)
	}

	f := &FakeTCP{
		lport:       locPort,
		rport:       remPort,
		seqOverhead: opt.seqOverhead,
	}
	f.seq.Store(opt.initSeq)
	f.ack.Store(opt.initAck)
	if opt.pseudoSum1Set {
		f.pseudoSum1 = new(uint16)
		*f.pseudoSum1 = opt.pseudoSum1
	}
	return f
}

// SendAttach input tcp payload, attach tcp header, and return
// tcp packet.
func (f *FakeTCP) SendAttach(seg *relraw.Packet) {
	var hdr = make(header.TCP, header.TCPMinimumSize)
	hdr.Encode(&header.TCPFields{
		SrcPort:    f.lport,
		DstPort:    f.rport,
		SeqNum:     f.seq.Load(),
		AckNum:     f.ack.Load(),
		DataOffset: header.TCPMinimumSize,
		// todo: if ACK not increase，not set ack，otherwise: TCP segment of a reassembled PDU
		Flags:         header.TCPFlagPsh | header.TCPFlagAck,
		WindowSize:    0xff32, // todo: mock
		Checksum:      0,
		UrgentPointer: 0,
	})
	hdr[fakeFlagOffset] |= fakeFlag

	f.seq.Add(uint32(seg.Len()) + f.seqOverhead)

	seg.Attach(hdr)
	if f.pseudoSum1 != nil {
		tcp := header.TCP(seg.Data())
		psum := checksum.Combine(*f.pseudoSum1, uint16(len(tcp)))

		sum := checksum.Checksum(tcp, psum)
		tcp.SetChecksum(^sum)

		if debug.Debug() {
			test.ValidTCP(test.T(), seg.Data(), *f.pseudoSum1)
		}
	}

}

// RecvStrip input a tcp packet, update ack, and return
// tcp payload.
func (f *FakeTCP) RecvStrip(tcp *relraw.Packet) {
	hdr := header.TCP(tcp.Data())

	// actually no need the header anymore
	if debug.Debug() {
		hdr[fakeFlagOffset] ^= fakeFlag
		require.False(test.T(), IsFakeTCP(hdr))

		const sumDelta = uint16(fakeFlag) << 8
		sum := ^hdr.Checksum()
		sum = checksum.Combine(sum, ^sumDelta)
		hdr.SetChecksum(^sum)

		test.ValidTCP(test.T(), hdr, *f.pseudoSum1)
	}

	f.ack.Store(max(f.ack.Load(), hdr.SequenceNumber()))

	// remove tcp header
	tcp.SetHead(tcp.Head() + int(hdr.DataOffset()))
}

const (
	fakeFlagOffset = header.TCPDataOffset
	fakeFlag       = 0b10
)

func IsFakeTCP(tcphdr header.TCP) bool {
	return tcphdr[fakeFlagOffset]&fakeFlag == fakeFlag
}
