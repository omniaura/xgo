package goparse

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// file is optional
func ParseFileCode(file string, code []byte) (ast *ast.File, fset *token.FileSet, err error) {
	fset = token.NewFileSet()
	ast, err = parser.ParseFile(fset, file, code, parser.ParseComments)
	return
}
