package tasks

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// annotatefFile is the one file allowed to restate a cause through
// Errorf(CodeOf(cause), ...): wire.Annotatef, which restores the cause's
// not-retryable mark on the result.
const annotatefFile = "internal/tasks/wire/retry.go"

// rewrapSites returns the positions in src of each call Errorf(CodeOf(x), ...),
// qualified or not. Such a call keeps the cause's code but drops its
// WithoutRetry mark (CAL-V0-078, V1-0908); wire.Annotatef is the replacement.
func rewrapSites(fset *token.FileSet, name string, src []byte) ([]token.Position, error) {
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var sites []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || calleeName(call.Fun) != "Errorf" || len(call.Args) == 0 {
			return true
		}
		if first, ok := call.Args[0].(*ast.CallExpr); ok && calleeName(first.Fun) == "CodeOf" {
			sites = append(sites, fset.Position(call.Pos()))
		}
		return true
	})
	return sites, nil
}

func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

func TestCALV0078_RewrapControls(t *testing.T) {
	for src, want := range map[string]int{
		"package p\nfunc f(err error) error { return wire.Errorf(wire.CodeOf(err), \"w\", \"%v\", err) }\n":   1,
		"package wire\nfunc f(err error) error { return Errorf(CodeOf(err), \"w\", \"%v\", err) }\n":          1,
		"package p\nfunc f(err error) error { return wire.Annotatef(err, \"w\", \"%v\", err) }\n":             0,
		"package p\nfunc f(err error) error { return wire.Errorf(wire.CodeMalformed, \"w\", \"%v\", err) }\n": 0,
		"package p\nfunc f(err error) error { return fmt.Errorf(\"%s\", wire.CodeOf(err)) }\n":                0,
	} {
		sites, err := rewrapSites(token.NewFileSet(), "control.go", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if len(sites) != want {
			t.Errorf("%d rewrap sites in %q, want %d", len(sites), src, want)
		}
	}
}

// TestCALV0078_NoRewrapDropsNotRetryable (V1-2132): no non-test Go source in
// the module restates an error as Errorf(CodeOf(err), ...), which would drop
// the cause's not-retryable mark; such a site must use wire.Annotatef.
func TestCALV0078_NoRewrapDropsNotRetryable(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skipDir(path, d, root) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == annotatefFile {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(src, []byte("CodeOf(")) {
			return nil
		}
		sites, err := rewrapSites(fset, filepath.ToSlash(rel), src)
		if err != nil {
			return err
		}
		for _, site := range sites {
			t.Errorf("%s: Errorf(CodeOf(err), ...) drops the not-retryable mark; use wire.Annotatef (CAL-V0-078)", site)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
