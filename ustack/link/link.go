package link

import (
	"context"

	"github.com/lysShub/itun/ustack/link/nofin"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type LinkEndpoint interface {
	stack.LinkEndpoint

	InjectInbound(tcpip.NetworkProtocolNumber, stack.PacketBufferPtr)
	ReadContext(ctx context.Context) stack.PacketBufferPtr
	Close()
}

var _ LinkEndpoint = (*nofin.Endpoint)(nil)
var _ LinkEndpoint = (*channel.Endpoint)(nil)
