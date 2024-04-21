package capture

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"log/slog"

	"github.com/lysShub/fatun/app/client/filter"
	sess "github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// todo: Capture 需要解决IP DF问题
//
//	tcp 可以控制MSS opt解决，如果需要代理UDP，则无法完美解决，
//	sconn并未限制IP MF， 但是需要把Send/Recv的pkt的容量设置得
//	足够大即可。
//
// 所以现在只能优化MTU问题。
type Capture interface {
	Capture(ctx context.Context) (Session, error)
	Close() error
}

type Session interface {
	Capture(ctx context.Context, pkt *packet.Packet) (err error)
	Inject(pkt *packet.Packet) error

	ID() sess.Session
	String() string
	Close() error
}

func NewCapture(hit filter.Hitter, cfg *Config) (Capture, error) {
	return newCapture(hit, cfg)
}

type Config struct {
	// NIC  int
	// IPv6 bool

	Logger *slog.Logger

	Priority int16

	Mtu int
}

type MssWrap struct {
	Session
	delta int
	cnt   uint8
}

func WrapMss(child Session, delta int) Session {
	if child.ID().Proto != header.TCPProtocolNumber {
		return child
	}

	return &MssWrap{
		Session: child,
		delta:   delta,
	}
}

func (m *MssWrap) Capture(ctx context.Context, pkt *packet.Packet) (err error) {
	if err = m.Session.Capture(ctx, pkt); err != nil {
		return err
	}

	if m.cnt < 8 {
		UpdateMSS(pkt.Bytes(), m.delta)
		m.cnt++
	}
	return nil
}

func UpdateMSS(hdr header.TCP, delta int) error {
	n := int(hdr.DataOffset())
	if n > header.TCPMinimumSize && delta != 0 {
		oldSum := ^hdr.Checksum()

		// if invalid format packet, no-operate
		for i := header.TCPMinimumSize; i < n; {
			kind := hdr[i]
			switch kind {
			case header.TCPOptionMSS:
				/* {kind} {length} {max seg size} */
				if i+4 <= n && hdr[i+1] == 4 {
					old := binary.BigEndian.Uint16(hdr[i+2:])
					new := int(old) + delta
					if new <= 0 {
						return errors.Errorf("updated mss is invalid %d", new)
					}

					if (i+2)%2 == 0 {
						binary.BigEndian.PutUint16(hdr[i+2:], uint16(new))
						sum := checksum.Combine(checksum.Combine(oldSum, ^old), uint16(new))
						hdr.SetChecksum(^sum)
					} else if i+5 <= n {
						sum := checksum.Combine(oldSum, ^checksum.Checksum(hdr[i+1:i+5], 0))

						binary.BigEndian.PutUint16(hdr[i+2:], uint16(new))

						sum = checksum.Combine(sum, checksum.Checksum(hdr[i+1:i+5], 0))
						hdr.SetChecksum(^sum)
					}
					return nil
				} else {
					return errors.Errorf("invalid tcp packet: %s", hex.EncodeToString(hdr[:n]))
				}
			case header.TCPOptionNOP:
				i += 1
			case header.TCPOptionEOL:
				return nil // not mss opt
			default:
				if i+1 < n {
					i += int(hdr[i+1])
				} else {
					return errors.Errorf("invalid tcp packet: %s", hex.EncodeToString(hdr[:n]))
				}
			}
		}
	}
	return nil
}

func GetMSS(tcp header.TCP) uint16 {
	n := int(tcp.DataOffset())
	if n > header.TCPMinimumSize {
		for i := header.TCPMinimumSize; i < n; {
			kind := tcp[i]
			switch kind {
			case header.TCPOptionMSS:
				if i+4 <= n && tcp[i+1] == 4 {
					return binary.BigEndian.Uint16(tcp[i+2:])
				} else {
					return 0
				}
			case header.TCPOptionNOP:
				i += 1
			case header.TCPOptionEOL:
				return 0
			default:
				if i+1 < n {
					i += int(tcp[i+1])
				} else {
					return 0
				}
			}
		}
	}
	return 0
}
