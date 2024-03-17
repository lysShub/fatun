package app

import (
	"log/slog"

	"github.com/lysShub/itun/sconn"
)

type Config struct {
	sconn.Config
	Logger slog.Handler
}

type ErrTooManyInvalidPacket struct{}

func (e ErrTooManyInvalidPacket) Error() string {
	return "recv too many invalid packet"
}
