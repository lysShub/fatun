package cctx

import (
	"context"
	"time"
)

/*
	cctx: cancel context, 基本思想是将ctx,cancel合并于一起。

	最初是使用场景是：一个对象需要同时运行多个协程才能正常工作，其中一个协程出错将导致整个对象不可工作。

	弃用原因：ctx作用于动作函数，而不是其对象本身，parent cancel 只会中止子动作函数执行，而不会影响对应子
		对象。所以，不应该"存储" ctx。
*/

// context can cancel, only used by parallel task
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
