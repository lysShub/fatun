package fatun

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"slices"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/crypto"
	"github.com/pkg/errors"
)

type Config struct {
	*fatcp.Config

	Logger *slog.Logger
}

func (c *Config) Init() error {
	if err := c.Config.Init(); err != nil {
		return err
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

type Sign struct {
	PSS  PrevSegmets
	Sign fatcp.Handshake
}

var _ fatcp.Handshake = (*Sign)(nil)

func (s *Sign) Client(ctx context.Context, tcp net.Conn) (crypto.Key, error) {
	stop := context.AfterFunc(ctx, func() {
		tcp.SetDeadline(time.Now())
	})
	defer stop()

	if err := s.PSS.Client(ctx, tcp); err != nil {
		return crypto.Key{}, err
	}

	return s.Sign.Client(ctx, tcp)
}

func (s *Sign) Server(ctx context.Context, tcp net.Conn) (crypto.Key, error) {
	stop := context.AfterFunc(ctx, func() {
		tcp.SetDeadline(time.Now())
	})
	defer stop()

	if err := s.PSS.Server(ctx, tcp); err != nil {
		return crypto.Key{}, err
	}

	return s.Sign.Server(ctx, tcp)
}

type PrevSegmets [][]byte

type ErrPrevPacketInvalid int

func (e ErrPrevPacketInvalid) Error() string {
	return fmt.Sprintf("previous pakcet %d is invalid", e)
}

func (pss PrevSegmets) Size() int {
	n := 0
	for _, e := range pss {
		n += len(e)
	}
	return n
}

func (pss PrevSegmets) Marshal(to string) error {
	fh, err := os.Create(to)
	if err != nil {
		return err
	}
	defer fh.Close()

	var dst []byte
	for i, e := range pss {
		n := hex.EncodedLen(len(e))
		dst = slices.Grow(dst, n+1)
		dst = dst[:n+1]

		hex.Encode(dst, e)
		if i != len(pss)-1 {
			dst[n] = '\n'
			n = n + 1
		}
		if _, err = fh.Write(dst[:n]); err != nil {
			return err
		}
	}
	return nil
}

func (pss *PrevSegmets) Unmarshal(from string) error {
	fh, err := os.Open(from)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fh.Close()

	data, err := io.ReadAll(fh)
	if err != nil {
		return errors.WithStack(err)
	}

	ps := bytes.Split(data, []byte{'\n'})
	for i := range ps {
		n, err := hex.Decode(ps[i], ps[i])
		if err != nil {
			return errors.WithStack(err)
		}
		ps[i] = ps[i][:n]
	}
	(*pss) = ps

	return nil
}

func (pss PrevSegmets) Client(ctx context.Context, conn net.Conn) (err error) {

	for i := 0; i < len(pss); i++ {
		if i%2 == 0 {
			_, err := conn.Write(pss[i])
			if err != nil {
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				default:
					return errors.WithStack(err)
				}
			}
		} else {
			var b = make([]byte, len(pss[i]))

			if _, err := io.ReadFull(conn, b); err != nil {
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				default:
					return errors.WithStack(err)
				}
			}
			if !bytes.Equal(b, pss[i]) {
				return ErrPrevPacketInvalid(i)
			}
		}
	}
	return nil
}

func (pss PrevSegmets) Server(ctx context.Context, conn net.Conn) (err error) {
	for i := 0; i < len(pss); i++ {
		if i%2 == 0 {
			var b = make([]byte, len(pss[i]))

			if _, err := io.ReadFull(conn, b); err != nil {
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				default:
					return errors.WithStack(err)
				}
			}
			if !bytes.Equal(b, pss[i]) {
				return ErrPrevPacketInvalid(i)
			}
		} else {
			_, err := conn.Write(pss[i])
			if err != nil {
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				default:
					return errors.WithStack(err)
				}
			}
		}
	}
	return nil
}
