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

var KeepaliveExceeded = ErrkeepaliveExceeded{}

type ErrkeepaliveExceeded struct{}

func (ErrkeepaliveExceeded) Error() string   { return "keepalive exceeded" }
func (ErrkeepaliveExceeded) Timeout() bool   { return true }
func (ErrkeepaliveExceeded) Temporary() bool { return true }
