package app_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/lysShub/itun/app"
	pkge "github.com/pkg/errors"
)

func Test_Loggeer(t *testing.T) {

	l := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	err := c()

	l.LogAttrs(
		context.Background(), slog.LevelError,
		err.Error(), app.TraceAttr(err),
	)

	l.Error(err.Error(), app.TraceAttr(err))
}

func a() error {
	e := errors.New("xxx")
	return pkge.WithStack(e)
}

func b() error {
	e := a()
	if e != nil {
		return pkge.WithStack(e)
	}

	return nil
}

func c() error {
	e := b()
	if e != nil {
		err := pkge.WithStack(errors.New("c-fail"))

		return app.Join(e, err)
	}

	return nil
}
