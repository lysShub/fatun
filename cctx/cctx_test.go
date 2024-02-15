package cctx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/stretchr/testify/require"
)

func Test_Cancel_Ctx(t *testing.T) {

	t.Run("repeate-cancel", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(errors.New("1"))
		c.Cancel(errors.New("2"))

		<-c.Done()
	})

	t.Run("cancel-anyone", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		defer c.Cancel(nil)

		go func() {
			time.Sleep(time.Second)
			c.Cancel(errors.New("go1"))
		}()
		go func() {
			time.Sleep(time.Second)
			c.Cancel(errors.New("go2"))
		}()

		<-c.Done()
	})

	t.Run("cancel-time", func(t *testing.T) {
		c := cctx.WithTimeout(context.Background(), time.Second)
		defer c.Cancel(nil)

		go func() {
			time.Sleep(time.Second * 2)
			c.Cancel(errors.New("go1"))
		}()

		s := time.Now()
		<-c.Done()
		require.Less(t, time.Since(s), time.Second+time.Millisecond*500)
		require.Greater(t, time.Since(s), time.Millisecond*500)
	})

	t.Run("cancel-nil", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(nil)
		c.Cancel(errors.New("2"))

		e := c.Err()
		require.Equal(t, context.Canceled, e)
	})

	t.Run("ctx-cancel-error", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(errors.New("1"))
		c.Cancel(errors.New("2"))

		<-c.Done()
		require.Equal(t, errors.New("1"), c.Err())
	})

	t.Run("ctx-not-cancel-error", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		defer c.Cancel(nil)

		require.Equal(t, nil, c.Err())
	})

	t.Run("timectx-not-cancel-error", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		defer c.Cancel(nil)

		require.Equal(t, nil, c.Err())
	})

	t.Run("ctx-cancel-nil", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(nil)
		c.Cancel(errors.New("2"))

		<-c.Done()
		require.Equal(t, context.Canceled, c.Err())
	})

	t.Run("timectx-cancel-nil", func(t *testing.T) {
		c := cctx.WithTimeout(context.Background(), time.Second)
		c.Cancel(nil)
		c.Cancel(errors.New("2"))

		<-c.Done()
		require.Equal(t, context.Canceled, c.Err())
	})

	t.Run("timectx-cancel-timeout", func(t *testing.T) {
		c := cctx.WithTimeout(context.Background(), time.Second)

		time.Sleep(time.Second * 2)
		c.Cancel(errors.New("1"))

		<-c.Done()

		e := c.Err()
		require.Equal(t, context.DeadlineExceeded, e)
	})
}
