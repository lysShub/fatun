package link

import (
	"context"
	"net/netip"

	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Chan struct {
	*channel.Endpoint

	wn writeNotify

	recordSeqAck bool
	seq, ack     uint32
}

var _ Link = (*Chan)(nil)

func NewChan(size int, mtu int) *Chan {
	var c = &Chan{
		Endpoint: channel.New(size, uint32(mtu), ""),
		wn:       newWriteNotify(size),
	}
	c.Endpoint.AddNotify(c.wn)

	return c
}

func (c *Chan) Inbound(ip *relraw.Packet) {
	if c.recordSeqAck {
		hdrSize := 0
		proto := tcpip.TransportProtocolNumber(0)
		switch header.IPVersion(ip.Data()) {
		case 4:
			hdrSize = int(header.IPv4(ip.Data()).HeaderLength())
			proto = header.IPv4(ip.Data()).TransportProtocol()
		case 6:
			hdrSize = header.IPv6MinimumSize
			proto = header.IPv6(ip.Data()).TransportProtocol()
		default:
			panic("")
		}
		if proto == header.TCPProtocolNumber {
			ack := header.TCP(ip.Data()[hdrSize:]).AckNumber()
			c.ack = max(c.ack, ack)
		}
	}

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(ip.Data()),
	})

	c.Endpoint.InjectInbound(header.IPv4ProtocolNumber, pkb)
}

func (c *Chan) Outbound(ctx context.Context, ip *relraw.Packet) error {
	pkb := c.Endpoint.ReadContext(ctx)
	if pkb.IsNil() {
		return ctx.Err()
	}
	defer pkb.DecRef()

	if c.recordSeqAck {
		if pkb.TransportProtocolNumber == header.TCPProtocolNumber {
			seq := header.TCP(pkb.TransportHeader().Slice()).SequenceNumber()
			c.seq = max(seq, c.seq)
		}
	}

	ip.SetLen(pkb.Size())
	b := ip.Data()

	n := 0
	for _, e := range pkb.AsSlices() {
		n += copy(b[n:], e)
	}
	return nil
}

func (c *Chan) OutboundBy(ctx context.Context, dst netip.AddrPort, ip *relraw.Packet) error {
	var pkb *stack.PacketBuffer
	for pkb.IsNil() {
		pkb = c.walkBy(dst)
		if pkb.IsNil() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				c.wn.Wait()
			}
		}
	}
	defer pkb.DecRef()

	ip.SetLen(pkb.Size())
	b := ip.Data()

	n := 0
	for _, e := range pkb.AsSlices() {
		n += copy(b[n:], e)
	}
	return nil
}

func (c *Chan) walkBy(dst netip.AddrPort) (pkb *stack.PacketBuffer) {
	n := c.Endpoint.NumQueued()
	for i := 0; i < n; i++ {
		pkb = c.Endpoint.Read()
		if pkb.IsNil() {
			return nil
		}
		if match(pkb, dst) {
			break
		} else {
			var pkbs stack.PacketBufferList
			pkbs.PushBack(pkb)

			// will drop not too old pkb
			c.Endpoint.WritePackets(pkbs)
		}
	}
	return nil
}

func (c *Chan) SeqAck() (uint32, uint32) {
	c.recordSeqAck = true
	return c.seq, c.ack
}

type writeNotify chan struct{}

func newWriteNotify(size int) writeNotify {
	return make(writeNotify, size)
}

func (w writeNotify) WriteNotify() {
	select {
	case w <- struct{}{}:
	default:
	}
}

func (w writeNotify) Wait() {
	<-w
}
