package gocfg

import (
	"go/ast"
)

func init() {
	// ast.Walk()
}

type Walker struct {
	w Walk
}

func NewWalker(w Walk) *Walker {
	return &Walker{w: w}
}

type Walk interface {
	Decl(node ast.Decl) (next bool)
	Spec(node ast.Spec) (next bool)
	Expr(node ast.Expr) (next bool)
}

func (l *Walker) WalkDecl(decl ast.Decl) {
	switch e := decl.(type) {
	case *ast.GenDecl:
		if l.w.Decl(e) {
			for _, e := range e.Specs {
				l.WalkSpec(e)
			}
		}
	case *ast.FuncDecl:
	case *ast.BadDecl:
	default:
	}
}

func (l *Walker) WalkSpec(spec ast.Spec) {
	switch e := spec.(type) {
	case *ast.ImportSpec:
	case *ast.ValueSpec:
		if l.w.Spec(e) {
			if e.Type != nil {
				l.WalkExpr(e.Type)
			}
			for _, e := range e.Values {
				l.WalkExpr(e)
			}
		}
	case *ast.TypeSpec:
	default:
	}
}

func (l *Walker) WalkExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.BadExpr:
		l.w.Expr(e)
	case *ast.Ident:
		l.w.Expr(e)
	case *ast.Ellipsis:
		if l.w.Expr(e) {
			l.WalkExpr(e.Elt)
		}
	case *ast.BasicLit:
		l.w.Expr(e)
	case *ast.FuncLit:
		l.w.Expr(e)
	case *ast.CompositeLit:
		if l.w.Expr(e) {
			l.WalkExpr(e.Type)
			for _, e := range e.Elts {
				l.WalkExpr(e)
			}
		}
	case *ast.ParenExpr:
		if l.w.Expr(e) {
			l.WalkExpr(e.X)
		}
	case *ast.SelectorExpr:
	case *ast.IndexExpr:
	case *ast.IndexListExpr:
	case *ast.SliceExpr:
	case *ast.TypeAssertExpr:
	case *ast.CallExpr:
	case *ast.StarExpr:
	case *ast.UnaryExpr:
	case *ast.BinaryExpr:
	case *ast.KeyValueExpr:
	case *ast.ArrayType:
	case *ast.StructType:
	case *ast.FuncType:
	case *ast.InterfaceType:
	case *ast.MapType:
	case *ast.ChanType:
	default:
	}
}

type NotWalk struct {
}

var _ Walk = (*NotWalk)(nil)

func (t *NotWalk) Decl(node ast.Decl) (next bool) { return false }
func (t *NotWalk) Spec(node ast.Spec) (next bool) { return false }
func (t *NotWalk) Expr(node ast.Expr) (next bool) { return false }
