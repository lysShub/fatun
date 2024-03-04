package link

import (
	"context"
	"net/netip"
	"sync"

	// "github.com/lysShub/itun/ustack/link/ring"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type List struct {
	list listIface

	dispatcher   stack.NetworkDispatcher
	dispatcherMu sync.RWMutex

	mtu                int
	LinkEPCapabilities stack.LinkEndpointCapabilities
	SupportedGSOKind   stack.SupportedGSO

	recordSeqAck bool
	seq, ack     uint32
}

var _ Link = (*List)(nil)

func NewList(size int, mtu int) *List {
	size = max(size, 4)
	return &List{
		// list:         newHeap(size), // todo: heap can't pass ut, fix bug
		list:         newSlice(size),
		mtu:          mtu,
		recordSeqAck: true,
	}
}

var _ stack.LinkEndpoint = (*List)(nil)

func (l *List) SeqAck() (uint32, uint32) {
	l.recordSeqAck = false
	return l.seq, l.ack
}

func (l *List) Outbound(ctx context.Context, ip *relraw.Packet) error {
	pkb := l.list.Get(ctx)
	if pkb.IsNil() {
		return ctx.Err()
	}
	defer pkb.DecRef()

	if l.recordSeqAck {
		if pkb.TransportProtocolNumber == header.TCPProtocolNumber {
			seq := header.TCP(pkb.TransportHeader().Slice()).SequenceNumber()
			l.seq = max(seq, l.seq)
		}
	}

	ip.SetLen(pkb.Size())
	b := ip.Data()

	n := 0
	for _, e := range pkb.AsSlices() {
		n += copy(b[n:], e)
	}

	if debug.Debug() {
		require.Equal(test.T(), ip.Len(), n)
		test.ValidIP(test.T(), ip.Data())
	}
	return nil
}

func (l *List) OutboundBy(ctx context.Context, dst netip.AddrPort, ip *relraw.Packet) error {
	pkb := l.list.GetBy(ctx, dst)
	if pkb.IsNil() {
		return ctx.Err()
	}
	defer pkb.DecRef()

	ip.SetLen(pkb.Size())
	b := ip.Data()

	n := 0
	for _, e := range pkb.AsSlices() {
		n += copy(b[n:], e)
	}

	if debug.Debug() {
		require.Equal(test.T(), ip.Len(), n)
		// todo: valid dst addr
	}

	return nil
}

func (l *List) Inbound(ip *relraw.Packet) {
	if l.recordSeqAck {
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
			l.ack = max(l.ack, ack)
		}
	}

	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(ip.Data()),
	})

	l.dispatcherMu.RLock()
	d := l.dispatcher
	l.dispatcherMu.RUnlock()
	if d != nil {
		l.dispatcher.DeliverNetworkPacket(header.IPv4ProtocolNumber, pkb)
	}
}

func (l *List) WritePackets(pkts stack.PacketBufferList) (int, tcpip.Error) {
	n := 0
	for i, pkb := range pkts.AsSlice() {
		if debug.Debug() {
			require.Greater(test.T(), pkb.Size(), 0)
		}

		ok := l.list.Put(pkb)
		if !ok {
			if i == 0 {
				return 0, &tcpip.ErrNoBufferSpace{}
			}
			break
		}
		n++
	}
	return n, nil
}

func (l *List) ARPHardwareType() header.ARPHardwareType { return header.ARPHardwareNone }
func (l *List) AddHeader(*stack.PacketBuffer)           {}
func (l *List) Attach(dispatcher stack.NetworkDispatcher) {
	l.dispatcherMu.Lock()
	defer l.dispatcherMu.Unlock()
	l.dispatcher = dispatcher
}
func (l *List) Capabilities() stack.LinkEndpointCapabilities { return l.LinkEPCapabilities }
func (l *List) GSOMaxSize() uint32                           { return 1 << 15 }
func (l *List) IsAttached() bool {
	l.dispatcherMu.RLock()
	defer l.dispatcherMu.RUnlock()
	return l.dispatcher != nil
}
func (l *List) LinkAddress() tcpip.LinkAddress       { return "" }
func (l *List) MTU() uint32                          { return uint32(l.mtu) }
func (l *List) MaxHeaderLength() uint16              { return 0 }
func (l *List) NumQueued() int                       { return l.list.Size() }
func (l *List) ParseHeader(*stack.PacketBuffer) bool { return true }
func (l *List) SupportedGSO() stack.SupportedGSO     { return l.SupportedGSOKind }
func (l *List) Wait()                                {}

// func (e *List) Close()
// func (e *List) Drain() int
// func (e *Ring) AddNotify(notify Notification) *NotificationHandle
// func (e *Ring) RemoveNotify(handle *NotificationHandle)
// func (l *List) InjectInbound(protocol tcpip.NetworkProtocolNumber, pkt *stack.PacketBuffer)

type listIface interface {
	Put(pkb *stack.PacketBuffer) (ok bool)
	Get(ctx context.Context) (pkb *stack.PacketBuffer)
	GetBy(ctx context.Context, dst netip.AddrPort) (pkb *stack.PacketBuffer)
	Size() int
}

type slice struct {
	s  []*stack.PacketBuffer
	mu sync.RWMutex

	writeCh chan struct{}
}

func newSlice(size int) *slice {
	return &slice{
		s:       make([]*stack.PacketBuffer, 0, size),
		writeCh: make(chan struct{}, size),
	}
}

var _ listIface = (*slice)(nil)

func (s *slice) Put(pkb *stack.PacketBuffer) (ok bool) {
	if pkb.IsNil() {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.s) == cap(s.s) {
		return false
	} else {
		s.s = append(s.s, pkb.IncRef())
		select {
		case s.writeCh <- struct{}{}:
		default:
		}
		return true
	}
}

func (s *slice) Get(ctx context.Context) (pkb *stack.PacketBuffer) {
	pkb = s.get()
	if !pkb.IsNil() {
		return pkb
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.writeCh:
			pkb = s.get()
			if !pkb.IsNil() {
				return pkb
			}
		}
	}
}

func (s *slice) get() (pkb *stack.PacketBuffer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.s) > 0 {
		pkb = s.s[0]
		n := copy(s.s, s.s[1:])
		s.s = s.s[:n]
		return pkb
	} else {
		return nil
	}
}

func (s *slice) GetBy(ctx context.Context, dst netip.AddrPort) (pkb *stack.PacketBuffer) {
	pkb = s.getBy(dst)
	if !pkb.IsNil() {
		return pkb
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.writeCh:
			pkb = s.getBy(dst)
			if !pkb.IsNil() {
				return pkb
			}
		}
	}
}

func (s *slice) getBy(dst netip.AddrPort) (pkb *stack.PacketBuffer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.s) > 0 {
		for i, e := range s.s {
			if match(e, dst) {
				pkb = s.s[i]

				n := copy(s.s[i:], s.s[i+1:])
				s.s = s.s[:i+n]
				return pkb
			}
		}
	}
	return nil
}

func (s *slice) Size() int {
	return len(s.s)
}
