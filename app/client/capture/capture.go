package capture

import (
	"log/slog"
)

// Capture capture ip packet, for tcp, only read SYN packet, for udp, read everyone packet.
// when read a ip packet, will call Hit(ip), if return false, Capture will recover the packet on link.
type Capture interface {
	Close() error
}

type Client interface {
	Logger() *slog.Logger
	MTU() int
	DivertPriority() int16
	Hit(ip []byte) bool
}

func New(client Client) (Capture, error) {
	return newCapture(client)
}

// type Config struct {
// 	// NIC  int
// 	// IPv6 bool

// 	Logger *slog.Logger

// 	Priority int16

// 	MTU int
// }

// type MssWrap struct {
// 	Session
// 	delta int
// 	cnt   uint8
// }

// func WrapMss(child Session, delta int) Session {
// 	if child == nil || child.ID().Proto != header.TCPProtocolNumber {
// 		return child
// 	}

// 	return &MssWrap{
// 		Session: child,
// 		delta:   delta,
// 	}
// }

// func (m *MssWrap) Capture(ctx context.Context, pkt *packet.Packet) (err error) {
// 	if err = m.Session.Capture(ctx, pkt); err != nil {
// 		return err
// 	}

// 	if m.cnt < 8 {
// 		UpdateMSS(pkt.Bytes(), m.delta)
// 		m.cnt++
// 	}
// 	return nil
// }
