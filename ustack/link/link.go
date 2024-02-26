package link

import (
	"context"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// link endpoint for tcp
type LinkEndpoint interface {
	stack.LinkEndpoint

	InjectInbound(tcpip.NetworkProtocolNumber, stack.PacketBufferPtr)
	ReadContext(ctx context.Context) stack.PacketBufferPtr

	// FinRstFlag has send or recv tcp flag contain FIN or RST,
	// use after tcp close.
	FinRstFlag() <-chan struct{}

	Close()
}

type ErrTCPCloseTimeout struct{}

func (e ErrTCPCloseTimeout) Error() string {
	return "close ustack tcp connnection timeout"
}
