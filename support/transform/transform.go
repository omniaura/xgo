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

func (c *File) GetTypeDecl(name string) *ast.TypeSpec {
	return GetTypeDecl(c.Ast.Decls, name)
}

func (c *File) GetFuncDecl(name string) *ast.FuncDecl {
	return c.getDecl("", name)
}

func (c *File) getDecl(typeName string, name string) *ast.FuncDecl {
	decls := c.Ast.Decls
	for _, decl := range decls {
		fnDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if typeName != "" {
			if fnDecl.Recv == nil || len(fnDecl.Recv.List) == 0 {
				continue
			}
			t := fnDecl.Recv.List[0].Type
			if star, ok := t.(*ast.StarExpr); ok {
				t = star.X
			}
			nm, ok := t.(*ast.Ident)
			if !ok || nm.Name != typeName {
				continue
			}
		}
		if fnDecl.Name.Name == name {
			return fnDecl
		}
	}
	return nil
}

func GetTypeDecl(decls []ast.Decl, name string) *ast.TypeSpec {
	for _, decl := range decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name != nil && typeSpec.Name.Name == name {
				return typeSpec
			}
		}
	}
	return nil
}
