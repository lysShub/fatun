package fake

import (
	"sync/atomic"

	"github.com/lysShub/relraw"

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

func (f *FakeTCP) AttachSend(p *relraw.Packet) {
	var b = make(header.TCP, header.TCPMinimumSize) // todo: global
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
	f.seq.Add(uint32(p.Len()))

	b.SetChecksum(^checksum.Checksum(b, 0))

	p.Attach(b)
}

func (f *FakeTCP) AttachRecv(tcp *relraw.Packet) {
	tcphdr := header.TCP(tcp.Data())

	new := tcphdr.SequenceNumber() + uint32(len(tcphdr.Payload()))
	old := f.ack.Load()

	if new > old {
		f.ack.Store(new)
	}
}
