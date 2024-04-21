package gocfg

import (
	"go/ast"
	"go/token"
	"reflect"

	"github.com/pkg/errors"
)

func init() {
	// ast.Walk(nil, nil)
}

// func SpecType(spec ast.Spec) (lit string, kind reflect.Kind, err error) {
// 	var w = &walkSpecType{}
// 	NewWalker(w).WalkSpec(spec)
// 	return w.name, w.kind, w.err
// }

// SpecTypeVisitor get sepc node data type
type SpecTypeVisitor struct {
	Err  error
	Kind reflect.Kind
	Name string
}

func (v *SpecTypeVisitor) Visit(node ast.Node) (w ast.Visitor) {
	switch e := node.(type) {
	case *ast.ImportSpec:
		return nil
	case *ast.ValueSpec:
	case *ast.TypeSpec:

		v.Name = e.Name.Name
	default:
		return nil
	}

	return nil
}

type ExprKindVisitor struct {
	Err  error
	Kind reflect.Kind
}

func (v *ExprKindVisitor) Visit(node ast.Node) (w ast.Visitor) {
	switch e := node.(type) {
	case *ast.BadExpr:
	case *ast.Ident:
	case *ast.Ellipsis:
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			v.Kind = reflect.Int
		case token.FLOAT:
			v.Kind = reflect.Float64 // todo: platform relational
		case token.IMAG:
			v.Kind = reflect.Complex128
		case token.CHAR:
			v.Kind = reflect.Int32
		case token.STRING:
			v.Kind = reflect.String
		default:
			v.Err = errors.Errorf("invalid BasicLit kind %d", e.Kind)
		}
	case *ast.FuncLit:
		v.Kind = reflect.Func
	case *ast.CompositeLit:
		v.Visit(e.Type)
	case *ast.ParenExpr:
		v.Visit(e.X)
	case *ast.SelectorExpr:
	case *ast.IndexExpr:
	case *ast.IndexListExpr:
	case *ast.SliceExpr:
		v.Kind = reflect.Slice
	case *ast.TypeAssertExpr:
	case *ast.CallExpr:
	case *ast.StarExpr:
		v.Kind = reflect.Pointer
	case *ast.UnaryExpr:
		v.Visit(e.X)
	case *ast.BinaryExpr:
		v.Visit(e.X) // todo: incorrect, eg: (1+1)*1.5
	case *ast.KeyValueExpr:
		v.Visit(e.Value)
	case *ast.ArrayType:
		v.Visit(e.Elt)
	case *ast.StructType:
		v.Kind = reflect.Struct
	case *ast.FuncType:
		v.Kind = reflect.Func
	case *ast.InterfaceType:
		v.Kind = reflect.Interface
	case *ast.MapType:
		v.Kind = reflect.Map
	case *ast.ChanType:
		v.Kind = reflect.Chan
	default:
	}

	return nil
}

/*
func (t *walkSpecType) Spec(node ast.Spec) (next bool) {
	switch e := node.(type) {
	case *ast.ImportSpec:
		t.err = errors.New("not support ast.ImportSpec")
		return false
	case *ast.ValueSpec:

	case *ast.TypeSpec:
		t.Expr(e.Type)

		t.name = e.Name.Name
	default:
	}

	return false
}

func (t *walkSpecType) Expr(node ast.Expr) (next bool) {
	switch e := node.(type) {
	case *ast.ArrayType:
		if e.Len == nil { // slice

		} else {

		}
	case *ast.StructType:
	case *ast.FuncType:
	case *ast.InterfaceType:
	case *ast.MapType:
	case *ast.ChanType:
	default:
		return false
	}

	return false
}

type walkExprKind struct {
	NotWalk

	err  error
	kind reflect.Kind
}

func ExprKind(expr ast.Expr) (kind reflect.Kind, err error) {
	w := &walkExprKind{}
	NewWalker(w).WalkExpr(expr)
	return w.kind, w.err
}

func (t *walkExprKind) Expr(node ast.Expr) (next bool) {
	switch e := node.(type) {
	case *ast.Ident:
	case *ast.ParenExpr:
	case *ast.SelectorExpr:
	case *ast.StarExpr:
	case *ast.ArrayType:
		if e.Len == nil {
			t.kind = reflect.Slice
		} else {
			t.kind = reflect.Array
		}
	case *ast.StructType:
		t.kind = reflect.Struct
	case *ast.FuncType:
		t.kind = reflect.Func
	case *ast.InterfaceType:
		t.kind = reflect.Interface
	case *ast.MapType:
		t.kind = reflect.Map
	case *ast.ChanType:
		t.kind = reflect.Chan
	default:
		t.err = errors.New("unknow")
	}
	return false
}
*/
