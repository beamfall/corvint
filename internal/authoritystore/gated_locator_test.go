package authoritystore

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestRootLocatorGated_TSP_V0_003 proves the gated root-locator declaration
// in .corvint/gated-root-locators.json from this package's own non-test
// source, every build variant included: root locating (os.Getwd, the
// --show-toplevel query) happens only in the repository binding, whose
// methods run only on a binding; a binding exists only through
// bindCurrentRepository, called only after decodeRoot
// admits accepted-root.json read through protectedFiles; no exported
// function takes a protectedFiles, whose only non-test supplier is
// openProtectedFiles, reading under RootPath; and RootPath is the declared
// gate. A change that breaks any step fails here, and the planner then keeps
// this package a root locator once the declaration is withdrawn.
func TestRootLocatorGated_TSP_V0_003(t *testing.T) {
	files := parsePackage(t)
	functions := map[string]*ast.FuncDecl{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				functions[functionName(function)] = function
			}
		}
	}

	locating := map[string]bool{}
	for name, function := range functions {
		ast.Inspect(function, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				if qualifier, ok := node.X.(*ast.Ident); ok && qualifier.Name == "os" && node.Sel.Name == "Getwd" {
					locating[name] = true
				}
			case *ast.BasicLit:
				if value, err := strconv.Unquote(node.Value); err == nil && strings.Contains(value, "--show-"+"toplevel") {
					locating[name] = true
				}
			}
			return true
		})
	}
	wantLocating := []string{"bindCurrentRepository", "repositoryBinding.unchanged", "verifyGitRoot"}
	if got := sortedNames(locating); !slices.Equal(got, wantLocating) {
		t.Fatalf("root-locating functions = %v, want %v", got, wantLocating)
	}

	callers := func(callee string) []string {
		found := map[string]bool{}
		for name, function := range functions {
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					found[name] = found[name] || fun.Name == callee
				case *ast.SelectorExpr:
					found[name] = found[name] || fun.Sel.Name == callee
				}
				return true
			})
		}
		for name, hit := range found {
			if !hit {
				delete(found, name)
			}
		}
		return sortedNames(found)
	}
	if got := callers("verifyGitRoot"); !slices.Equal(got, []string{"repositoryBinding.unchanged"}) {
		t.Fatalf("verifyGitRoot callers = %v", got)
	}
	wantBinders := []string{"resolveLifecycleContextProfile", "resolveObservedProfile"}
	if got := callers("bindCurrentRepository"); !slices.Equal(got, wantBinders) {
		t.Fatalf("bindCurrentRepository callers = %v, want %v", got, wantBinders)
	}
	for _, name := range wantBinders {
		decoded, bound := callPosition(functions[name], "decodeRoot"), callPosition(functions[name], "bindCurrentRepository")
		if decoded == token.NoPos || decoded > bound {
			t.Fatalf("%s binds the repository before decodeRoot admits the protected root", name)
		}
		if !readsRootDocument(functions[name], decoded) {
			t.Fatalf("%s does not read accepted-root.json through protectedFiles before decodeRoot", name)
		}
	}

	suppliers := []string{}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CompositeLit:
				if isNamed(node.Type, "repositoryBinding") && !inFunction(file, node.Pos(), functions["bindCurrentRepository"]) {
					t.Errorf("a repositoryBinding is built outside bindCurrentRepository at %v", node.Pos())
				}
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "new" && len(node.Args) == 1 && isNamed(node.Args[0], "repositoryBinding") {
					t.Errorf("new(repositoryBinding) at %v", node.Pos())
				}
			case *ast.ValueSpec:
				if isNamed(node.Type, "repositoryBinding") {
					t.Errorf("a zero repositoryBinding is declared at %v", node.Pos())
				}
			case *ast.FuncDecl:
				if node.Type.Results != nil {
					for _, result := range node.Type.Results.List {
						if isNamed(result.Type, "protectedFiles") {
							suppliers = append(suppliers, functionName(node))
						}
					}
				}
				if node.Name.IsExported() && node.Type.Params != nil {
					for _, parameter := range node.Type.Params.List {
						if isNamed(parameter.Type, "protectedFiles") || isNamed(parameter.Type, "repositoryBinding") {
							t.Errorf("exported %s takes a protectedFiles or repositoryBinding", functionName(node))
						}
					}
				}
			}
			return true
		})
	}
	for _, supplier := range suppliers {
		if supplier != "openProtectedFiles" {
			t.Errorf("%s supplies a protectedFiles; only openProtectedFiles may", supplier)
		}
	}
	read, ok := functions["darwinFiles.read"]
	if !ok || !callsWithIdent(read, "Join", "RootPath") {
		t.Fatal("darwinFiles.read does not resolve every read under RootPath")
	}

	body, err := os.ReadFile("../../.corvint/gated-root-locators.json")
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Packages map[string]struct {
			Gate string `json:"gate"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	if entry, ok := declared.Packages["internal/authoritystore"]; !ok || entry.Gate != RootPath || !filepath.IsAbs(RootPath) {
		t.Fatalf("declared gate %q, RootPath %q", entry.Gate, RootPath)
	}
}

func parsePackage(t *testing.T) []*ast.File {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatal("no package sources")
	}
	return files
}

// functionName is a function's name, or Receiver.Method for a method. A
// name declared once per build variant (openProtectedFiles) maps once.
func functionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return ident.Name + "." + function.Name.Name
	}
	return "?." + function.Name.Name
}

func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isNamed(expression ast.Expr, name string) bool {
	if star, ok := expression.(*ast.StarExpr); ok {
		expression = star.X
	}
	ident, ok := expression.(*ast.Ident)
	return ok && ident.Name == name
}

func callPosition(function *ast.FuncDecl, callee string) token.Pos {
	position := token.NoPos
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && position == token.NoPos {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == callee {
				position = call.Pos()
			}
		}
		return true
	})
	return position
}

// readsRootDocument reports a files.read("accepted-root.json", ...) call
// before position.
func readsRootDocument(function *ast.FuncDecl, before token.Pos) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || call.Pos() >= before || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "read" || !isNamed(selector.X, "files") {
			return true
		}
		if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Value == strconv.Quote("accepted-root.json") {
			found = true
		}
		return true
	})
	return found
}

func inFunction(file *ast.File, position token.Pos, function *ast.FuncDecl) bool {
	return function != nil && slices.Contains(file.Decls, ast.Decl(function)) && position >= function.Pos() && position < function.End()
}

func callsWithIdent(function *ast.FuncDecl, callee, argument string) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != callee {
			return true
		}
		for _, arg := range call.Args {
			if isNamed(arg, argument) {
				found = true
			}
		}
		return true
	})
	return found
}
