package config

import (
	"go/parser"
	"go/token"
	"log/slog"
	"os"

	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/sconn"
	"github.com/pkg/errors"
)

// var _ = (&printer.Config{}).Fprint(nil, nil, *ast.File)

type Config struct {
	Server string

	PrevPackets string

	SwapKey sconn.SwapKey

	MTU int

	Log string
}

func (cfg *Config) Config() (*app.Config, error) {
	scfg := &sconn.Config{
		SwapKey: cfg.SwapKey,
		MTU:     cfg.MTU,
	}
	err := scfg.PrevPackets.Unmarshal(cfg.PrevPackets)
	if err != nil {
		return nil, err
	}

	var fh *os.File
	switch cfg.Log {
	case "stderr":
		fh = os.Stderr
	case "stdout", "":
		fh = os.Stdout
	default:
		fh, err = os.Create(cfg.Log)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	c := &app.Config{
		Config: scfg,
		MTU:    cfg.MTU,
		Logger: slog.NewJSONHandler(fh, nil),
	}
	return c, nil
}

func (cfg *Config) Load(from string) error {
	panic("todo: ")

	mode := parser.ParseComments | parser.DeclarationErrors | parser.AllErrors
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, from, nil, mode)
	if err != nil {
		return err
	}

	// var b = bytes.NewBuffer(nil)
	// (&printer.Config{
	// 	Mode:     printer.TabIndent | printer.UseSpaces,
	// 	Tabwidth: 8,
	// }).Fprint(b, fs, f)
	// fh, err := os.Create(from + ".bak")
	// if err != nil {
	// 	return err
	// }
	// fh.Write(b.Bytes())

	_ = f

	return nil
}

func (cfg *Config) Flush(to string) error {
	return nil
}
