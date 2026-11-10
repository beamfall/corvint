// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
)

// tempJoinComponents proves the literals of one Go file that name no
// repository path because they are joined only to a test-scoped temporary
// root (AFP-V0-9810): a string argument, other than the first, of
// `filepath.Join` or `path.Join` whose first argument is temp-rooted. An
// expression is temp-rooted when it is a call of `TempDir()` on a function
// parameter of type `*testing.T`, `*testing.B`, `*testing.F` or `testing.TB`
// (the imported "testing" package under its local name), a call of
// `os.TempDir()`, such a join itself, or a local variable bound once, by `:=`
// or a single-name `var` with one value, to a temp-rooted expression and
// never assigned again, addressed or declared at package level. A test that
// writes `filepath.Join(t.TempDir(), "docs", "guide.md")` builds a fixture
// below the temporary directory, which lies outside every checkout, so the
// literal names no read of `docs/guide.md` in the repository. Only such
// arguments are returned, as the body offsets of their literals; every other
// literal keeps AFP-V0-012's rule (c), including a literal spelled elsewhere
// in the file, a component of `..`, `.`, an empty or glob component, a join
// whose root is any other expression, and every literal of a file that does
// not parse. aliases maps the file's local import names to import paths.
func tempJoinComponents(body []byte, aliases map[string]string) map[int]bool {
	if !bytes.Contains(body, []byte("TempDir")) {
		return nil
	}
	testing, osName, joins := "", "", map[string]bool{}
	for local, importPath := range aliases {
		switch importPath {
		case "testing":
			testing = local
		case "os":
			osName = local
		case "path/filepath", "path":
			joins[local] = true
		}
	}
	if testing == "" && osName == "" || len(joins) == 0 {
		return nil
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "", body, 0)
	shift := 0
	if err != nil {
		const prefix = "package tempjoin\n"
		positions = token.NewFileSet()
		file, err = parser.ParseFile(positions, "", append([]byte(prefix), body...), 0)
		shift = len(prefix)
	}
	if err != nil {
		return nil
	}
	// parameters holds every function parameter whose type is a testing
	// value; bindings every local variable's single binding expression.
	parameters := map[*ast.Object]bool{}
	bindings := map[*ast.Object]ast.Expr{}
	assigned := map[*ast.Object]int{}
	refused := map[*ast.Object]bool{}
	testingType := func(expr ast.Expr) bool {
		if testing == "" {
			return false
		}
		if star, ok := expr.(*ast.StarExpr); ok {
			expr = star.X
		}
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Obj != nil || qualifier.Name != testing {
			return false
		}
		switch selector.Sel.Name {
		case "T", "B", "F", "TB":
			return true
		}
		return false
	}
	bind := func(name *ast.Ident, value ast.Expr) {
		if name == nil || name.Obj == nil || name.Name == "_" {
			return
		}
		assigned[name.Obj]++
		if _, seen := bindings[name.Obj]; seen || value == nil {
			refused[name.Obj] = true
			return
		}
		bindings[name.Obj] = value
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncType:
			if n.Params == nil {
				return true
			}
			for _, field := range n.Params.List {
				if !testingType(field.Type) {
					continue
				}
				for _, name := range field.Names {
					if name.Obj != nil {
						parameters[name.Obj] = true
					}
				}
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE && len(n.Lhs) == len(n.Rhs) {
				for index, left := range n.Lhs {
					if name, ok := left.(*ast.Ident); ok {
						bind(name, n.Rhs[index])
					}
				}
				return true
			}
			for _, left := range n.Lhs {
				if name, ok := left.(*ast.Ident); ok && name.Obj != nil {
					assigned[name.Obj]++
					refused[name.Obj] = true
				}
			}
		case *ast.GenDecl:
			if n.Tok != token.VAR {
				return true
			}
			for _, spec := range n.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(value.Names) == 1 && len(value.Values) == 1 {
					bind(value.Names[0], value.Values[0])
					continue
				}
				for _, name := range value.Names {
					if name.Obj != nil {
						refused[name.Obj] = true
					}
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				if name, ok := n.X.(*ast.Ident); ok && name.Obj != nil {
					refused[name.Obj] = true
				}
			}
		}
		return true
	})
	// A package-level binding is visible to every function, including one
	// that reassigns it through a closure the walk above does not model.
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range value.Names {
					if name.Obj != nil {
						refused[name.Obj] = true
					}
				}
			}
		}
	}
	visiting := map[*ast.Object]bool{}
	var tempRooted func(expr ast.Expr) bool
	joinCall := func(expr ast.Expr) (*ast.CallExpr, bool) {
		call, ok := expr.(*ast.CallExpr)
		if !ok || call.Ellipsis.IsValid() || len(call.Args) < 2 {
			return nil, false
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Join" {
			return nil, false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Obj != nil || !joins[qualifier.Name] {
			return nil, false
		}
		return call, true
	}
	tempRooted = func(expr ast.Expr) bool {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			return tempRooted(e.X)
		case *ast.Ident:
			if e.Obj == nil || refused[e.Obj] || assigned[e.Obj] != 1 || visiting[e.Obj] {
				return false
			}
			value, bound := bindings[e.Obj]
			if !bound {
				return false
			}
			visiting[e.Obj] = true
			defer delete(visiting, e.Obj)
			return tempRooted(value)
		case *ast.CallExpr:
			if call, ok := joinCall(e); ok {
				if !tempRooted(call.Args[0]) {
					return false
				}
				for _, argument := range call.Args[1:] {
					if _, ok := boundedJoinArgument(argument); !ok {
						return false
					}
				}
				return true
			}
			selector, ok := e.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "TempDir" || len(e.Args) != 0 || e.Ellipsis.IsValid() {
				return false
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return false
			}
			if receiver.Obj == nil {
				return osName != "" && receiver.Name == osName
			}
			return parameters[receiver.Obj] && assigned[receiver.Obj] == 0 && !refused[receiver.Obj]
		}
		return false
	}
	components := map[int]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		expr, ok := node.(ast.Expr)
		if !ok {
			return true
		}
		call, ok := joinCall(expr)
		if !ok || !tempRooted(call.Args[0]) {
			return true
		}
		for _, argument := range call.Args[1:] {
			literal, ok := argument.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			if _, plain := boundedJoinArgument(argument); !plain {
				return true
			}
		}
		for _, argument := range call.Args[1:] {
			components[positions.Position(argument.Pos()).Offset-shift] = true
		}
		return true
	})
	if len(components) == 0 {
		return nil
	}
	return components
}
