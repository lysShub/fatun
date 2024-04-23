package session

import (
	"encoding/binary"
	"encoding/hex"
	"log/slog"

	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client interface {
	Uplink(pkt *packet.Packet, id session.ID) error
	Logger() *slog.Logger
	MTU() int
	DivertPriority() int16
	Release(session.ID)
}

type Session interface {
	Inject(pkt *packet.Packet) error
	Close() error
}

func UpdateMSS(hdr header.TCP, delta int) error {
	n := int(hdr.DataOffset())
	if n > header.TCPMinimumSize && delta != 0 {
		oldSum := ^hdr.Checksum()
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
