package nofin

import (
	"context"
	"sync/atomic"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Endpoint struct {
	*channel.Endpoint

	seq, ack atomic.Uint32
}

func (e *Endpoint) setSeq(seq uint32) {
	if seq > e.seq.Load() {
		e.seq.Store(seq)
	}
}

func (e *Endpoint) setAck(ack uint32) {
	e.ack.Store(ack)
}

func (e *Endpoint) SeqAck() (seq, ack uint32) {
	return e.seq.Load(), e.ack.Load()
}

// Read read gvisor's outboud ip packet
func (e *Endpoint) ReadContext(ctx context.Context) stack.PacketBufferPtr {
	pkt := e.Endpoint.ReadContext(ctx)

	if !pkt.IsNil() && pkt.TransportProtocolNumber == header.TCPProtocolNumber {
		tcphdr := header.TCP(pkt.TransportHeader().Slice())

		e.setSeq(tcphdr.SequenceNumber())
		EncodeCustomFIN(tcphdr)
	}

	return pkt
}

// Inject inject tcp packet to gvistor stack.
func (e *Endpoint) InjectInbound(protocol tcpip.NetworkProtocolNumber, pkt stack.PacketBufferPtr) {
	ss := pkt.AsSlices() // avoid memcpy
	if len(ss) == 1 {
		var tcphdr header.TCP
		switch header.IPVersion(ss[0]) {
		case 4:
			iphdr := header.IPv4(ss[0])
			if iphdr.TransportProtocol() == header.TCPProtocolNumber {
				tcphdr = iphdr.Payload()
			}
		case 6:
			iphdr := header.IPv6(ss[0])
			if iphdr.TransportProtocol() == header.TCPProtocolNumber {
				tcphdr = iphdr.Payload()
			}
		default:
		}
		if len(tcphdr) > 0 {
			DecodeCustomFIN(tcphdr)
			e.setAck(tcphdr.SequenceNumber())
		}
	} else {
		e.decodeSlices(ss, protocol)
	}

	e.Endpoint.InjectInbound(protocol, pkt)
}

func (e *Endpoint) decodeSlices(bs [][]byte, proto tcpip.NetworkProtocolNumber) {
	var trans tcpip.TransportProtocolNumber
	var tcpIdx int
	switch proto {
	case header.IPv4ProtocolNumber:
		tcpIdx = int(header.IPv4(bs[0]).HeaderLength())
		if t, ok := getSlices(bs, 9); !ok {
			return
		} else {
			trans = tcpip.TransportProtocolNumber(t)
		}
	case header.IPv6ProtocolNumber:
		tcpIdx = header.IPv6MinimumSize
		if t, ok := getSlices(bs, header.IPv6NextHeaderOffset); !ok {
			return
		} else {
			trans = tcpip.TransportProtocolNumber(t)
		}
	default:
		return
	}
	if trans != header.TCPProtocolNumber {
		return
	}

	s, s2 := slicesIdx(bs, tcpIdx)
	var tcphdr = make(header.TCP, 0, header.TCPMinimumSize)

	tcphdr = append(tcphdr, bs[s][s2:]...)
	for i := s + 1; i < len(bs) && len(tcphdr) > header.TCPMinimumSize; i++ {
		tcphdr = append(tcphdr, bs[i]...)
	}

	if DecodeCustomFIN(tcphdr) {
		j := copy(bs[s][s2:], tcphdr)
		for i := s + 1; i < len(bs) && j < len(tcphdr); i++ {
			j += copy(bs[i], tcphdr[j:])
		}
	}
}

func slicesIdx(ss [][]byte, I int) (idx, idx2 int) {
	for i := 0; idx < len(ss); idx++ {
		i += len(ss[idx])
		if i > I {
			idx2 = len(ss[idx]) - (i - I)
			return
		}
	}
	return -1, -1
}

func getSlices(ss [][]byte, idx int) (byte, bool) {
	i, i2 := slicesIdx(ss, idx)
	if i < 0 {
		return 0, false
	}
	return ss[i][i2], true
}
