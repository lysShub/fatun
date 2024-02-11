package fake

import (
	"itun/sconn/crypto"
	"math/rand"
	"sync/atomic"

	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

/*
	tcp 伪装栈：会模仿传输过程中tcp的栈, 关心seq和ack，不会有wnd的管理，
				不会有阻塞流控，不会有handshake，close步骤， 不会有缓存，
				不会异步操作, tcp头不会有option.
				checksum是没有计算pso的。

*/

type FakeTCP struct {
	lport, rport uint16
	seq          atomic.Uint32
	ack          atomic.Uint32

	// todo:
	// raw []byte
}

// NewFakeTCP set fake tcp header
func NewFakeTCP(localPort, remotePort uint16, initSeq, initAck uint32) *FakeTCP {
	f := &FakeTCP{
		lport: localPort,
		rport: remotePort,
	}
	f.seq.Store(initSeq)
	f.ack.Store(initAck)
	return f
}

func init() {
	var tcphdr header.TCP
	tcphdr.Encode(&header.TCPFields{
		SrcPort:    19986,
		DstPort:    8080,
		SeqNum:     rand.Uint32(),
		AckNum:     rand.Uint32(),
		DataOffset: header.TCPMinimumSize,
		Flags:      0,
		WindowSize: uint16(rand.Uint32()),
		Checksum:   0,
	})
}

func (f *FakeTCP) Send(b []byte, reserved int) (tcp []byte, empty int) {
	i := reserved - header.TCPMinimumMSS

	if i <= 0 {
		n := (len(b) - reserved) + header.IPv4MinimumSize + header.TCPMinimumSize
		tmp := make([]byte, n, n+crypto.Bytes)
		copy(tmp[header.TCPMinimumSize+header.IPv4MinimumSize:], b[reserved:])
		b, i = tmp, header.IPv4MinimumSize
	}
	tcphdr := header.TCP(b[i:])

	tcphdr.Encode(&header.TCPFields{
		SrcPort:    f.lport,
		DstPort:    f.rport,
		SeqNum:     f.seq.Load(),
		AckNum:     f.ack.Load(),
		DataOffset: header.TCPMinimumSize,
		Flags:      header.TCPFlagPsh | header.TCPFlagAck,
		WindowSize: 0xff32, // todo: rand
		Checksum:   0,
	})
	f.seq.Add(uint32(len(b) - reserved))

	tcphdr.SetChecksum(^checksum.Checksum(tcphdr, 0))

	return b, i
}

func (f *FakeTCP) Recv(tcp header.TCP) {
	f.ack.Store(tcp.SequenceNumber() + uint32(len(tcp.Payload()))) // todo: store greater
}
