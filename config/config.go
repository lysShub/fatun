package config

import (
	"fmt"
	"go/parser"
	"go/token"
	"time"

	"github.com/lysShub/fatun/sconn"
)

// var _ = (&printer.Config{}).Fprint(nil, nil, *ast.File)

type Config struct {

	// client set first tcp packet, server recv and check it, then replay
	// second tcp packet, etc.
	PrevPackets sconn.PrevPackets //todo: support mutiple data set

	HandShakeTimeout time.Duration

	// swap secret key
	SwapKey sconn.SwapKey
}

func (cfg *Config) Load(from string) error {
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

	fmt.Println(f)

	return nil
}

func (cfg *Config) Flush(to string) error {
	return nil
}
