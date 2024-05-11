package server

import (
	"log/slog"

	sconn "github.com/lysShub/fatcp"
)

type Config struct {
	sconn.Config

	Logger *slog.Logger
}

func (c *Config) Init() error {
	if c == nil {
		panic("nil config")
	}

	if err := c.Config.Init(); err != nil {
		return err
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}
