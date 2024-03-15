package conn

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/crypto"
)

type Config struct {
	Warner *slog.Logger

	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets config.PrevPackets //todo: support mutiple data set

	HandShakeTimeout time.Duration

	// swap secret key
	SwapKey crypto.SecretKey

	MTU  uint16
	IPv6 bool
}

func (c *Config) init() error {
	if c == nil {
		return errors.New("xx")
	}

	if c.Warner == nil {
		c.Warner = slog.New(slog.NewJSONHandler(os.Stdout, nil).WithGroup("proxy").WithAttrs([]slog.Attr{
			// {Key: "src", Value: slog.StringValue(raw.LocalAddrPort().String())},
		}))
	}

	return nil
}
