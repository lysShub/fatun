package ctx

import (
	"context"

	"go.uber.org/zap"
)

type Ctx struct {
	context.Context

	zap.Logger
}

func (c *Ctx) CondDo(f func()) {
	f()
}
