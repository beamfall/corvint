package bridge

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestConstructorsReturnUntypedNilError pins that every registry constructor
// returns a plain nil error on success, so a caller holding the result in an
// error variable does not see a typed-nil *Error as a failure (V1-3916).
func TestConstructorsReturnUntypedNilError(t *testing.T) {
	root := makeRepository(t)
	for name, constructor := range map[string]func(string) (*Registry, error){
		"default": New, "task-review": NewTaskReview, "task-review-lsp": NewTaskReviewLSP, "flows": NewFlows, "import-graph": NewImportGraph,
	} {
		registry, err := constructor(root)
		if err != nil || registry == nil {
			t.Fatalf("%s: registry=%v err=%#v", name, registry, err)
		}
		var asInterface error = err
		if asInterface != nil {
			t.Fatalf("%s: success returned a typed-nil error %#v", name, asInterface)
		}
		if _, err := constructor(filepath.Join(root, "missing")); err == nil {
			t.Fatalf("%s: a missing root was accepted", name)
		} else if failure, ok := err.(*Error); !ok || failure.Code != "invalid-root" {
			t.Fatalf("%s: failure=%#v, want *Error invalid-root", name, err)
		}
	}
}

// TestExportedConstructorsDeclareErrorResult pins that no exported
// package-level New* function in this package declares a *Error result,
// the shape that produced the V1-3916 typed-nil error.
func TestExportedConstructorsDeclareErrorResult(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, parsed := range packages {
		for _, file := range parsed.Files {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "New") || function.Type.Results == nil {
					continue
				}
				checked++
				for _, result := range function.Type.Results.List {
					if star, ok := result.Type.(*ast.StarExpr); ok {
						if ident, ok := star.X.(*ast.Ident); ok && ident.Name == "Error" {
							t.Errorf("%s declares a *Error result; return error instead (V1-3916)", function.Name.Name)
						}
					}
				}
			}
		}
	}
	if checked < 5 {
		t.Fatalf("checked %d constructors, want at least 5", checked)
	}
}

// bridgeCode is the bridge failure code of a constructor error, or "" for nil
// or a foreign error.
func bridgeCode(err error) string {
	if failure, ok := err.(*Error); ok && failure != nil {
		return failure.Code
	}
	return ""
}
