package fatun

import (
	"encoding/binary"
	"encoding/hex"
	"log/slog"

	"github.com/lysShub/fatun/sconn"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	*sconn.Config
	MTU    int
	Logger *slog.Logger
}

type ErrRecvTooManyError struct{}

func (e ErrRecvTooManyError) Error() string {
	return "recv too many invalid packet"
}

type ErrkeepaliveExceeded struct{}

func (ErrkeepaliveExceeded) Error() string   { return "keepalive exceeded" }
func (ErrkeepaliveExceeded) Timeout() bool   { return true }
func (ErrkeepaliveExceeded) Temporary() bool { return true }

type ErrNotRecord struct{}

func (ErrNotRecord) Error() string   { return "filter not record" }
func (ErrNotRecord) Temporary() bool { return true }

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
