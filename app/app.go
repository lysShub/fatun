package app

import (
	"log/slog"

	"github.com/lysShub/fatun/sconn"
)

type Config struct {
	sconn.Config
	MTU    int
	Logger slog.Handler
}

type ErrRecvTooManyError struct{}

func (e ErrRecvTooManyError) Error() string {
	return "recv too many invalid packet"
}
