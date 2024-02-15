package cctx

import (
	"context"
	"errors"
	"time"
)

// self cancel-able context, sub-goroutine can cancel
// whole task with error. the cctx.Err() return error from
// context.Cause(ctx) firstly, then ctx.Err().
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
	if err := context.Cause(c.Context); err != nil {
		return err
	}
	return c.Context.Err()
}

func WithContext(ctx context.Context) CancelCtx {
	var c = &cancelCtx{}
	c.Context, c.cancel = context.WithCancelCause(ctx)
	return c
}

type timeCancelCtx struct {
	context.Context
	cancel context.CancelFunc
	cause  *errWrap
}

func WithTimeout(ctx context.Context, timeout time.Duration) CancelCtx {
	var c = &timeCancelCtx{cause: &errWrap{}}
	c.Context, c.cancel = context.WithTimeoutCause(ctx, timeout, c.cause)
	return c
}

func WithDeadline(ctx context.Context, deadline time.Time) CancelCtx {
	var c = &timeCancelCtx{cause: &errWrap{}}
	c.Context, c.cancel = context.WithDeadlineCause(ctx, deadline, c.cause)
	return c
}

type errWrap struct {
	_   [0]func() // uncomparable
	err error
}

func (e *errWrap) Is(err error) bool {
	if err == nil {
		return e.err == nil
	}

	if se, ok := err.(*errWrap); ok {
		err = se.err
	}
	return errors.Is(err, e.err)
}

func (e *errWrap) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return ""
}

func (tc *timeCancelCtx) Cancel(cause error) {
	select {
	case <-tc.Context.Done():
	default:
		tc.cause.err = cause
		tc.cancel()
	}
}

func (tc *timeCancelCtx) Err() error {
	err := context.Cause(tc.Context)
	if err != nil && !errors.Is(err, &errWrap{}) {
		return err
	}

	return tc.Context.Err()
}
