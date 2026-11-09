// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// testMetadataLiterals proves direct string arguments consumed only as testing.T
// diagnostics or as an unobserved subtest name (AFP-V0-3334). It never drops a
// nested expression, a custom receiver, or a name a callback can read or pass to
// another function. Unparsed bodies keep every literal.
func testMetadataLiterals(body []byte, imports []*ast.ImportSpec) map[int]bool {
	aliases, _ := qualifiedImports(imports, func(p string) bool { return p == "testing" })
	if len(aliases) == 0 {
		return nil
	}
	const prefix = "package testmetadata\n"
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "", append([]byte(prefix), body...), 0)
	if err != nil {
		return nil
	}
	parameters := map[*ast.Object]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if function, ok := n.(*ast.FuncType); ok && function.Params != nil {
			for _, field := range function.Params.List {
				pointer, ok := field.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				typ, ok := pointer.X.(*ast.SelectorExpr)
				if !ok || typ.Sel.Name != "T" {
					continue
				}
				pkg, ok := typ.X.(*ast.Ident)
				if !ok || pkg.Obj != nil || aliases[pkg.Name] == "" {
					continue
				}
				for _, name := range field.Names {
					if name.Obj != nil {
						parameters[name.Obj] = true
					}
				}
			}
		}
		return true
	})
	metadata := map[int]bool{}
	mark := func(expr ast.Expr) {
		if literal, ok := expr.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			metadata[positions.Position(literal.Pos()).Offset-len(prefix)] = true
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := method.X.(*ast.Ident)
		if !ok || !parameters[receiver.Obj] {
			return true
		}
		if testDiagnostic(method.Sel.Name) {
			for _, arg := range call.Args {
				mark(arg)
			}
		} else if method.Sel.Name == "Run" && len(call.Args) == 2 {
			callback, ok := call.Args[1].(*ast.FuncLit)
			if ok && unobservedTestName(callback, parameters) {
				mark(call.Args[0])
			}
		}
		return true
	})
	return metadata
}

func testDiagnostic(method string) bool {
	switch method {
	case "Error", "Errorf", "Fatal", "Fatalf", "Log", "Logf", "Skip", "Skipf":
		return true
	}
	return false
}

// A callback that uses its handle only for diagnostics, Helper, or Parallel
// cannot derive a repository path from its name. Anything else is unproved.
func unobservedTestName(callback *ast.FuncLit, parameters map[*ast.Object]bool) bool {
	if callback.Type.Params == nil || len(callback.Type.Params.List) != 1 {
		return false
	}
	field := callback.Type.Params.List[0]
	if len(field.Names) != 1 || !parameters[field.Names[0].Obj] {
		return false
	}
	object := field.Names[0].Obj
	allowed := map[*ast.Ident]bool{}
	ast.Inspect(callback.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := method.X.(*ast.Ident)
		if ok && receiver.Obj == object && (testDiagnostic(method.Sel.Name) || method.Sel.Name == "Helper" || method.Sel.Name == "Parallel") {
			allowed[receiver] = true
		}
		return true
	})
	safe := true
	ast.Inspect(callback.Body, func(n ast.Node) bool {
		if name, ok := n.(*ast.Ident); ok && name.Obj == object && !allowed[name] {
			safe = false
		}
		return true
	})
	return safe
}
