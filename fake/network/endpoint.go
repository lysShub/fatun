package network

import (
	"sync/atomic"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type NetworkEndpoint interface {
	stack.NetworkEndpoint
	stack.AddressableEndpoint
}

type endpoint struct {
	NetworkEndpoint

	// *SeqAck
	seq, ack atomic.Uint32
}

// inbound ip packet
func (e *endpoint) HandlePacket(pkt stack.PacketBufferPtr) {
	ss := pkt.AsSlices()
	if len(ss) == 1 {
		switch header.IPVersion(ss[0]) {
		case 4:
			iphdr := header.IPv4(ss[0])
			if iphdr.TransportProtocol() == header.TCPProtocolNumber {
				DeCustomFIN(header.TCP(iphdr.Payload()))
			}
		case 6:
			iphdr := header.IPv4(ss[0])
			if iphdr.TransportProtocol() == header.TCPProtocolNumber {
				DeCustomFIN(header.TCP(iphdr.Payload()))
			}
		default:
		}
	} else {
		e.update(ss, ipHdrLen(ss), false)
	}

	e.NetworkEndpoint.HandlePacket(pkt)
}

// outbound tcp packet
func (e *endpoint) WritePacket(r *stack.Route, params stack.NetworkHeaderParams, pkt stack.PacketBufferPtr) tcpip.Error {
	if pkt.IsNil() {
		return nil
	}

	if params.Protocol == header.TCPProtocolNumber {
		ss := pkt.AsSlices()
		if len(ss) == 1 {
			tcphdr := header.TCP(ss[0])

			e.seq.Store(tcphdr.SequenceNumber())
			if tcphdr.Flags().Contains(header.TCPFlagAck) {
				e.ack.Store(tcphdr.AckNumber())
			}

			EnCustomFIN(tcphdr)
		} else {
			e.update(ss, 0, true)
		}
	}

	return e.NetworkEndpoint.WritePacket(r, params, pkt)
}

func ipHdrLen(ss [][]byte) int {
	switch header.IPVersion(ss[0]) {
	case 4:
		var tmp = make(header.IPv4, 0, header.ICMPv4MinimumSize)
		for i := 0; i < len(ss) && len(tmp) < header.ICMPv4MinimumSize; i++ {
			tmp = append(tmp, ss[i]...)
		}
		if tmp.TransportProtocol() == header.TCPProtocolNumber {
			return int(tmp.HeaderLength())
		}
	case 6:
		var tmp = make(header.IPv6, 0, header.IPv6MinimumSize)
		for i := 0; i < len(ss) && len(tmp) < header.ICMPv4MinimumSize; i++ {
			tmp = append(tmp, ss[i]...)
		}
		if tmp.TransportProtocol() == header.TCPProtocolNumber {
			return header.IPv6MinimumSize
		}
	}
	return -1
}

func (e *endpoint) update(ss [][]byte, tcpIdx int, encode bool) {
	if tcpIdx < 0 {
		return
	}

	idx, subIdx := 0, 0
	for i := 0; idx < len(ss); idx++ {
		i += len(ss[idx])
		if i > tcpIdx {
			subIdx = i - tcpIdx - len(ss[idx])
			break
		}
	}

	var tcphdr = make(header.TCP, 0, header.TCPMinimumSize)

	tcphdr = append(tcphdr, ss[idx][subIdx:]...)
	for i := idx + 1; i < len(ss) && len(tcphdr) > header.TCPMinimumSize; i++ {
		tcphdr = append(tcphdr, ss[i]...)
	}

	update := false
	if encode {
		update = EnCustomFIN(tcphdr)
		e.seq.Store(tcphdr.SequenceNumber())
		if tcphdr.Flags().Contains(header.TCPFlagAck) {
			e.ack.Store(tcphdr.AckNumber())
		}
	} else {
		update = DeCustomFIN(tcphdr)
	}

	if update {
		j := copy(ss[idx][subIdx:], tcphdr)
		for i := idx + 1; i < len(ss) && j < len(tcphdr); i++ {
			j += copy(ss[i], tcphdr[j:])
		}
	}
}

func (e *endpoint) Stats() stack.NetworkEndpointStats {
	// ss := e.NetworkEndpoint.Stats()
	return nil
}
