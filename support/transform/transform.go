// transform go code from one to another

package transform

import (
	"go/ast"
	"go/token"
)

type File struct {
	Code    []byte
	Ast     *ast.File
	AbsFile string
	Fset    *token.FileSet
}
