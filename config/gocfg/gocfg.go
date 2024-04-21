package gocfg

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"

	"github.com/pkg/errors"
)

func LoadFile[T string | []byte](file T, dst any) error {

	return nil
}

// Load src to dst
func Load(src *ast.Decl, dst any) error {

	return nil
}

type Unmarshaler struct {
	fset *token.FileSet
	file *ast.File

	dst         any
	dstTypeName string
}

const mode = parser.ParseComments | parser.DeclarationErrors | parser.AllErrors

// NewUnmarshal load go config file to dst.
func NewUnmarshal(file []byte, dst any) (*Unmarshaler, error) {
	var l = &Unmarshaler{
		fset: token.NewFileSet(),
		dst:  dst,
	}

	var err error
	if l.file, err = parser.ParseFile(l.fset, "", file, mode); err != nil {
		return nil, errors.WithStack(err)
	}

	v := reflect.ValueOf(l.dst)
	if v.IsNil() {
		return nil, errors.New("require dst isn't nil")
	} else if !v.IsValid() {
		return nil, errors.New("invalid dst")
	} else if v.Kind() != reflect.Pointer {
		return nil, errors.New("require dst is pointer data type")
	} else {
		v = reflect.Indirect(v)
		if v.Kind() == reflect.Pointer {
			return nil, errors.New("invalid dst data type")
		}
		l.dstTypeName = v.Type().Name()
	}
	return l, nil
}

func (l *Unmarshaler) findExpectDeclNode() (ast.Decl, error) {

	return nil, nil
}

func (l *Unmarshaler) Unmarshal() error {
	return nil
}

type findExpectDeclNodeWalker struct {
	expectName string
}

func (w *findExpectDeclNodeWalker) Decl(node ast.Decl) (next bool) {
	switch node.(type) {
	case *ast.GenDecl:
		return true
	default:
		return false
	}
}
func (w *findExpectDeclNodeWalker) Spec(node ast.Spec) (next bool) {
	switch node.(type) {
	case *ast.ValueSpec:
		return true
	default:
		return false
	}
}
func (w *findExpectDeclNodeWalker) Expr(node ast.Expr) (next bool) {
	switch node.(type) {
	case *ast.ArrayType:
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

var _ Walk = (*findExpectDeclNodeWalker)(nil)
