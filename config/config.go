package config

import (
	"log/slog"
	"time"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
)

type Config struct {
	Log slog.Handler

	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets sconn.PrevPackets //todo: support mutiple data set

	HandShakeTimeout time.Duration

	// swap secret key
	SwapKey crypto.SecretKey
}
