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

	t.Run("cancel/repeate-cancel", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(errors.New("1"))
		c.Cancel(errors.New("2"))

		<-c.Done()
	})

	t.Run("cancel/cancel-anyone", func(t *testing.T) {
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

	t.Run("cancel/timeout", func(t *testing.T) {
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

	t.Run("err/ctx-cancel-nil", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(nil)
		c.Cancel(errors.New("2"))

		<-c.Done()
		e := c.Err()
		require.Equal(t, context.Canceled, e)
	})
	t.Run("err/timectx-cancel-nil", func(t *testing.T) {
		c := cctx.WithTimeout(context.Background(), time.Hour)
		c.Cancel(nil)
		c.Cancel(errors.New("2"))

		<-c.Done()
		e := c.Err()
		require.Equal(t, context.Canceled, e)
	})

	t.Run("err/ctx-cancel-error", func(t *testing.T) {
		c := cctx.WithContext(context.Background())
		c.Cancel(errors.New("1"))
		c.Cancel(errors.New("2"))

		<-c.Done()
		require.Equal(t, errors.New("1"), c.Err())
	})
	t.Run("err/timectx-cancel-error", func(t *testing.T) {
		c := cctx.WithTimeout(context.Background(), time.Hour)
		c.Cancel(errors.New("1"))
		c.Cancel(errors.New("2"))

		<-c.Done()
		require.Equal(t, errors.New("1"), c.Err())
	})

	t.Run("err/timectx-timeout", func(t *testing.T) {
		c := cctx.WithTimeout(context.Background(), time.Second)

		time.Sleep(time.Second * 2)
		c.Cancel(errors.New("1"))

		<-c.Done()

		e := c.Err()
		require.Equal(t, context.DeadlineExceeded, e)
	})
}

func Test_Cancel_Ctx_Inherit(t *testing.T) {

	t.Run("child-ctx-err", func(t *testing.T) {

		p := cctx.WithContext(context.Background())
		c := cctx.WithContext(p)

		err := errors.New("a")
		p.Cancel(err)

		<-c.Done()
		<-p.Done()

		e := c.Err()
		require.True(t, errors.Is(err, e))
	})

}
