package cctx

import (
	"context"
	"time"
)

// self cancel-able context, sub-goroutine can cancel whole task with error.
// the cctx.Err() return error from context.Cause(ctx) firstly, then ctx.Err().
//
// if a function takes xxx as a parameter, it does not need to return err
type CancelCtx interface {
	context.Context
	Cancel(cause error)
}

type cancelCtx struct {
	context.Context
	cancel context.CancelCauseFunc
}

var _ CancelCtx = (*cancelCtx)(nil)

func (c *cancelCtx) Cancel(cause error) {
	select {
	case <-c.Context.Done():
	default:
		c.cancel(cause)
	}
}

func (c *cancelCtx) Err() error {
	err := context.Cause(c.Context)
	if err == nil {
		err = c.Context.Err()
	}
	return err
}

func WithContext(ctx context.Context) CancelCtx {
	var c = &cancelCtx{}
	c.Context, c.cancel = context.WithCancelCause(ctx)
	return c
}

type timeCancelCtx struct {
	context.Context
	cancel context.CancelFunc
	cause  error
}

func WithTimeout(ctx context.Context, timeout time.Duration) CancelCtx {
	var c = &timeCancelCtx{}
	c.Context, c.cancel = context.WithTimeoutCause(ctx, timeout, c.cause)
	return c
}

func WithDeadline(ctx context.Context, deadline time.Time) CancelCtx {
	var c = &timeCancelCtx{}
	c.Context, c.cancel = context.WithDeadlineCause(ctx, deadline, c.cause)
	return c
}

func (tc *timeCancelCtx) Cancel(cause error) {
	select {
	case <-tc.Context.Done():
	default:
		tc.cause = cause
		tc.cancel()
	}
}

func (tc *timeCancelCtx) Err() error {
	if tc.cause != nil {
		return tc.cause
	}
	return tc.Context.Err()
}
