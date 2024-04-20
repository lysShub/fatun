package faketcp

import (
	"math/rand"
	"sync/atomic"

	"github.com/lysShub/fatun/sconn/crypto"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"github.com/stretchr/testify/require"

	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// record tcp seq/ack, not care handshake/clode, etc.
// todo: more reasonable wnd
type FakeTCP struct {
	lport, rport uint16
	sndNxt       atomic.Uint32
	rcvNxt       atomic.Uint32

	pseudoSum1 *uint16
	crypto     crypto.Crypto
}

// PseudoSum1 if set, will calc tcp header checksum
func PseudoSum1(s uint16) func(f *FakeTCP) {
	return func(opt *FakeTCP) { opt.pseudoSum1 = &s }
}

func Crypto(c crypto.Crypto) func(f *FakeTCP) {
	return func(f *FakeTCP) { f.crypto = c }
}

// New set fake tcp header
func New(localPort, remotePort uint16, opts ...func(*FakeTCP)) *FakeTCP {
	var f = &FakeTCP{
		lport: localPort,
		rport: remotePort,
	}
	f.sndNxt.Store(rand.Uint32())
	f.rcvNxt.Store(rand.Uint32())
	for _, e := range opts {
		e(f)
	}

	return f
}

func (f *FakeTCP) InitNxt(snd, rcv uint32) {
	f.sndNxt.Store(snd)
	f.rcvNxt.Store(rcv)
}

func (f *FakeTCP) Overhead() int {
	n := 20 // ipv4 header
	if f.crypto != nil {
		n += f.crypto.Overhead()
	}
	return n
}

// AttachSend input tcp payload, attach tcp header, and return
// tcp packet.
func (f *FakeTCP) AttachSend(seg *packet.Packet) {
	payload := seg.Data()
	var hdr = make(header.TCP, header.TCPMinimumSize)
	hdr.Encode(&header.TCPFields{
		SrcPort:    f.lport,
		DstPort:    f.rport,
		SeqNum:     f.sndNxt.Load(),
		AckNum:     f.rcvNxt.Load(),
		DataOffset: header.TCPMinimumSize,
		// todo: if ACK not increase，not set ack，otherwise: TCP segment of a reassembled PDU
		Flags:         header.TCPFlagPsh | header.TCPFlagAck,
		WindowSize:    0xff32, // todo: mock
		Checksum:      0,
		UrgentPointer: 0,
	})
	hdr[fakeFlagOffset] |= fakeFlag
	seg.Attach(hdr)

	if f.crypto != nil {
		f.sndNxt.Add(uint32(payload + f.crypto.Overhead()))
		f.crypto.Encrypt(seg)
	} else if f.pseudoSum1 != nil {
		f.sndNxt.Add(uint32(payload))

		tcp := header.TCP(seg.Bytes())
		psum := checksum.Combine(*f.pseudoSum1, uint16(len(tcp)))

		sum := checksum.Checksum(tcp, psum)
		tcp.SetChecksum(^sum)

		if debug.Debug() {
			test.ValidTCP(test.T(), seg.Bytes(), *f.pseudoSum1)
		}
	}
}

// DetachRecv input a tcp packet, update ack, and return
// tcp payload.
func (f *FakeTCP) DetachRecv(tcp *packet.Packet) error {
	if f.crypto != nil {
		if err := f.crypto.Decrypt(tcp); err != nil {
			return errorx.WrapTemp(err)
		}
	} else if f.pseudoSum1 != nil {
		if debug.Debug() {
			hdr := header.TCP(tcp.Bytes())
			require.False(test.T(), Is(hdr))
			test.ValidTCP(test.T(), hdr, *f.pseudoSum1)
		}
	}

	hdr := header.TCP(tcp.Bytes())
	f.rcvNxt.Store(max(f.rcvNxt.Load(), hdr.SequenceNumber()+uint32(len(hdr.Payload()))))
	// f.ack.Store(max(f.ack.Load(), hdr.SequenceNumber()))

	// remove tcp header
	tcp.SetHead(tcp.Head() + int(hdr.DataOffset()))
	return nil
}

const (
	// todo: 区分fakeFlag采用tcp MSS option, 有为fake packet

	fakeFlagOffset = header.TCPDataOffset
	fakeFlag       = 0b10
)

func Is(tcphdr header.TCP) bool {
	return tcphdr[fakeFlagOffset]&fakeFlag == fakeFlag
}
