package gocfg

import (
	"fmt"
	"go/ast"
	"go/parser"
	"testing"
)

func TestXxxxx(t *testing.T) {
	for _, e := range exprs {
		e()
	}
}

var (
	exprs = []func() ast.Expr{
		exprBadExpr,
		exprIdent,
		exprEllipsis,
		exprBasicLit,
		exprFuncLit,
		exprCompositeLit,
		exprParenExpr,
		exprSelectorExpr,
		exprIndexExpr,
		// exprIndexListExpr,
		exprSliceExpr,
		exprTypeAssertExpr,
		exprCallExpr,
		exprStarExpr,
		exprUnaryExpr,
		exprBinaryExpr,
		exprKeyValueExpr,
		exprArrayType,
		exprStructType,
		exprFuncType,
		exprInterfaceType,
		exprMapType,
		exprChanType,
	}

	exprBadExpr = func() ast.Expr {
		return &ast.BadExpr{From: 1, To: 2}
	}
	exprIdent = func() ast.Expr {
		e := mustParseExpr("int")
		return e.(*ast.Ident)
	}
	exprEllipsis = func() ast.Expr {
		e := mustParseExpr("[...]int")
		// e := mustParseExpr("func(a ...int)")
		return (e.(*ast.ArrayType).Len).(*ast.Ellipsis) // todo: Elt not nil
	}
	exprBasicLit = func() ast.Expr {
		e := mustParseExpr("'1'")
		return e.(*ast.BasicLit)
	}
	exprFuncLit = func() ast.Expr {
		e := mustParseExpr("func(a int)error{return nil}")
		return e.(*ast.FuncLit)
	}
	exprCompositeLit = func() ast.Expr {
		e := mustParseExpr("struct{age int}{age:1}")
		return e.(*ast.CompositeLit)
	}
	exprParenExpr = func() ast.Expr {
		e := mustParseExpr("(1)")
		return e.(*ast.ParenExpr)
	}
	exprSelectorExpr = func() ast.Expr {
		e := mustParseExpr(`fmt.x`)
		return e.(*ast.SelectorExpr)
	}
	exprIndexExpr = func() ast.Expr {
		e := mustParseExpr(`s[1]`)
		return e.(*ast.IndexExpr)
	}
	// exprIndexListExpr = func() ast.Expr {
	//  // isn't normal expr, ref go\internal\typeparams\typeparams.go:PackIndexExpr
	// 	e := mustParseExpr(`a[1,2]`)
	// 	return e.(*ast.IndexListExpr)
	// }
	exprSliceExpr = func() ast.Expr {
		e := mustParseExpr(`s[1:2]`)
		return e.(*ast.SliceExpr)
	}
	exprTypeAssertExpr = func() ast.Expr {
		e := mustParseExpr(`e.(type)`)
		return e.(*ast.TypeAssertExpr)
	}
	exprCallExpr = func() ast.Expr {
		e := mustParseExpr(`print(1)`)
		return e.(*ast.CallExpr)
	}
	exprStarExpr = func() ast.Expr {
		e := mustParseExpr(`*int`)
		return e.(*ast.StarExpr)
	}
	exprUnaryExpr = func() ast.Expr {
		e := mustParseExpr(`-1`)
		return e.(*ast.UnaryExpr)
	}
	exprBinaryExpr = func() ast.Expr {
		e := mustParseExpr(`2-1`)
		return e.(*ast.BinaryExpr)
	}
	exprKeyValueExpr = func() ast.Expr {
		e := mustParseExpr(`T{a:1}`)
		return (e.(*ast.CompositeLit).Elts[0]).(*ast.KeyValueExpr)
	}
	exprArrayType = func() ast.Expr {
		e := mustParseExpr(`[2]int`)
		return e.(*ast.ArrayType)
	}
	exprStructType = func() ast.Expr {
		e := mustParseExpr(`struct{age int}`)
		return e.(*ast.StructType)
	}
	exprFuncType = func() ast.Expr {
		e := mustParseExpr(`func(int)error`)
		return e.(*ast.FuncType)
	}
	exprInterfaceType = func() ast.Expr {
		e := mustParseExpr(`interface{Add()}`)
		return e.(*ast.FuncType)
	}
	exprMapType = func() ast.Expr {
		e := mustParseExpr(`map[int]string`)
		return e.(*ast.MapType)
	}
	exprChanType = func() ast.Expr {
		e := mustParseExpr(`chan<- int`)
		return e.(*ast.ChanType)
	}
)

func mustParseExpr(e string) ast.Expr {
	expr, err := parser.ParseExpr(e)
	if err != nil {
		panic(fmt.Sprintf(`"%s": %s`, e, err.Error()))
	}
	return expr
}
