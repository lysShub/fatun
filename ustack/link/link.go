package link

import (
	"context"

	"github.com/lysShub/relraw"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// link endpoint for tcp
type LinkEndpoint interface {
	stack.LinkEndpoint

	Inbound(ip *relraw.Packet)
	Outbound(ctx context.Context, ip *relraw.Packet)
	Close()

	// todo: 复用stack, 不在需要close stack
	//
	// FinRstFlag has send or recv tcp flag contain FIN or RST,
	// use after tcp close.
	FinRstFlag() <-chan struct{}
}

type ErrTCPCloseTimeout struct{}

func (e ErrTCPCloseTimeout) Error() string {
	return "close ustack tcp connnection timeout"
}
