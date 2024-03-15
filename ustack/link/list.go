package link

import (
	"context"
	"io"
	"net/netip"
	"sync"

	"github.com/pkg/errors"

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
}

var _ Link = (*List)(nil)

func NewList(size int, mtu int) *List {
	size = max(size, 4)
	return &List{
		// list:         newHeap(size), // todo: heap can't pass ut, fix bug
		list: newSlice(size),
		mtu:  mtu,
	}
}

var _ stack.LinkEndpoint = (*List)(nil)

func (l *List) Outbound(ctx context.Context, tcp *relraw.Packet) error {
	pkb := l.list.Get(ctx)
	if pkb.IsNil() {
		return errors.WithStack(ctx.Err())
	}
	defer pkb.DecRef()

	if pkb.Size() > tcp.Cap() {
		return errors.WithStack(io.ErrShortBuffer)
	}
	tcp.SetLen(pkb.Size())
	data := tcp.Data()

	n := 0
	for _, e := range pkb.AsSlices() {
		n += copy(data[n:], e)
	}

	if debug.Debug() {
		test.ValidIP(test.T(), tcp.Data())
	}
	switch pkb.NetworkProtocolNumber {
	case header.IPv4ProtocolNumber:
		hdrLen := header.IPv4(tcp.Data()).HeaderLength()
		tcp.SetHead(int(hdrLen))
	case header.IPv6ProtocolNumber:
		tcp.SetHead(header.IPv6MinimumSize)
	default:
		panic("")
	}
	return nil
}

func (l *List) OutboundBy(ctx context.Context, dst netip.AddrPort, tcp *relraw.Packet) error {
	pkb := l.list.GetBy(ctx, dst)
	if pkb.IsNil() {
		return errors.WithStack(ctx.Err())
	}
	defer pkb.DecRef()

	if pkb.Size() > tcp.Cap() {
		return errors.WithStack(io.ErrShortBuffer)
	}
	tcp.SetLen(pkb.Size())
	data := tcp.Data()

	n := 0
	for _, e := range pkb.AsSlices() {
		n += copy(data[n:], e)
	}

	if debug.Debug() {
		test.ValidIP(test.T(), tcp.Data())
	}
	switch pkb.NetworkProtocolNumber {
	case header.IPv4ProtocolNumber:
		hdrLen := header.IPv4(tcp.Data()).HeaderLength()
		tcp.SetHead(int(hdrLen))
	case header.IPv6ProtocolNumber:
		tcp.SetHead(header.IPv6MinimumSize)
	default:
		panic("")
	}

	return nil
}

func (l *List) Inbound(ip *relraw.Packet) {
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

	for i, e := range s.s {
		if match(e, dst) {
			pkb = s.s[i]

			n := copy(s.s[i:], s.s[i+1:])
			s.s = s.s[:i+n]
			return pkb
		}
	}
	return nil
}

func (s *slice) Size() int {
	return len(s.s)
}
