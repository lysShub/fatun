package fatun

import (
	"log/slog"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/rawsock/test"
)

type Config struct {
	*fatcp.Config

	Logger *slog.Logger
}

func (c *Config) Init() error {
	if err := c.Config.Init(test.LocIP()); err != nil {
		return err
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}
