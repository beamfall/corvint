package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// TestDeclarations is what one Go test file declares and uses, read for a
// test-function slice (STR-V0-063): the top-level test functions `go test
// -run` can name, every other top-level name another file of the package
// could reach, every identifier the file mentions, and whether the file runs
// code before any test (an `init` or `TestMain` function, a package-level
// variable whose initializer calls something, or a blank or dot import), and
// the import paths the file names (STR-V0-071).
type TestDeclarations struct {
	Tests      []string
	Others     []string
	Uses       []string
	Imports    []string
	SideEffect string
}

// TestFileDeclarations parses one `_test.go` file. The lists are sorted and
// without duplicates; a parse error is returned as is, so the caller keeps
// the package whole.
func TestFileDeclarations(name string, src []byte) (TestDeclarations, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		return TestDeclarations{}, err
	}
	var declarations TestDeclarations
	tests, others, uses, imports := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			name := decl.Name.Name
			switch {
			case decl.Recv != nil:
				others[name] = true
			case name == "init" || name == "TestMain":
				declarations.SideEffect = "declares " + name
				others[name] = true
			case isTestFunction(name):
				tests[name] = true
			default:
				others[name] = true
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ImportSpec:
					// A blank import exists only for the imported package's
					// initialization; a dot import makes its exported names
					// part of the file's scope, which Uses cannot attribute.
					path, err := strconv.Unquote(spec.Path.Value)
					if err != nil {
						path = spec.Path.Value
					}
					imports[path] = true
					if spec.Name != nil && declarations.SideEffect == "" {
						switch spec.Name.Name {
						case "_":
							declarations.SideEffect = "imports " + path + " for its side effects"
						case ".":
							declarations.SideEffect = "dot-imports " + path
						}
					}
				case *ast.TypeSpec:
					others[spec.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						others[name.Name] = true
					}
					if decl.Tok == token.VAR && declarations.SideEffect == "" {
						for _, value := range spec.Values {
							if calls(value) {
								declarations.SideEffect = "initializes " + spec.Names[0].Name + " with a call"
							}
						}
					}
				}
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok {
			uses[ident.Name] = true
		}
		return true
	})
	delete(others, "_")
	declarations.Tests, declarations.Others, declarations.Uses, declarations.Imports = sortedNames(tests), sortedNames(others), sortedNames(uses), sortedNames(imports)
	return declarations, nil
}

// isTestFunction reports a top-level name `go test -run` matches: a Test,
// Example or Fuzz function. Benchmarks need -bench and are not run by either
// a whole or a sliced invocation.
func isTestFunction(name string) bool {
	for _, prefix := range []string{"Test", "Example", "Fuzz"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && (rest == "" || !isLower(rest[0])) {
			return true
		}
	}
	return false
}

func isLower(c byte) bool { return 'a' <= c && c <= 'z' }

// calls reports whether an expression contains a call: a value computed at
// package initialization, which every test of the package observes.
func calls(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if _, ok := node.(*ast.CallExpr); ok {
			found = true
		}
		return !found
	})
	return found
}

func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
