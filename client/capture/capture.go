package capture

import (
	"log/slog"

	"github.com/lysShub/netkit/packet"
)

// Capture capture ip packet, for tcp, only read SYN packet, for udp, read everyone packet.
// when read a ip packet, will call Hit(ip), if return false, Capture will recover the packet on link.
type Capture interface {
	Close() error
}

type Client interface {
	Logger() *slog.Logger
	MaxRecvBuffSize() int
	DivertPriority() int16
	Hit(ip *packet.Packet) bool
}

func New(client Client) (Capture, error) {
	return newCapture(client)
}
