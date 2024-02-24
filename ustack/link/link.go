package link

import (
	"context"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type LinkEndpoint interface {
	stack.LinkEndpoint

	InjectInbound(tcpip.NetworkProtocolNumber, stack.PacketBufferPtr)
	ReadContext(ctx context.Context) stack.PacketBufferPtr
	Close()
}
