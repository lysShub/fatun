package app

import (
	"log/slog"

	"github.com/lysShub/itun/sconn"
)

type Config struct {
	sconn.Config
	Logger slog.Handler
}

type ErrRecvTooManyError struct{}

func (e ErrRecvTooManyError) Error() string {
	return "recv too many invalid packet"
}
