package gocfg

import (
	"fmt"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXxx(t *testing.T) {
	var f = `
		package a

		import(
			"io"
			"errors"
		)

		var(
			x int
			y int
		)

		var a = A{age:1}
		var b = struct{name string}{name:"xxx"}

		type A struct{
			age int
		}
	`

	expr, err := parser.ParseFile(token.NewFileSet(), "", f, mode)
	require.NoError(t, err)
	fmt.Println(expr)
}
