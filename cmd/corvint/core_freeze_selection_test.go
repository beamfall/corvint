package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTSPV0961CoreVerbChangeKeepsFrozenProfileTest pins TSP-V0-961: a TSP-V0-006 slice of
// cmd/corvint for a change to a Core verb's own source never drops
// TestCoreVerbsEmitTheFrozenProfiles. For the first function of each Core-verb file whose
// signature is all basic types (the shape TSP-V0-006 can slice), a body change keeps the package
// whole or keeps the frozen test in the slice, and never runs `-run ^$`.
func TestTSPV0961CoreVerbChangeKeepsFrozenProfileTest(t *testing.T) {
	const frozen = "TestCoreVerbsEmitTheFrozenProfiles"
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range []string{"affected.go", "prove.go", "ocm.go", "frontier.go"} {
		rel := "cmd/corvint/" + name
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		fn := firstBasicFunction(file)
		if fn == nil {
			continue
		}
		at := fset.Position(fn.Body.Lbrace).Offset + 1
		base := string(src[:at]) + "\n\t_ = 0" + string(src[at:])
		candidate := testSourceCandidate{pkg: "github.com/Beamfall/corvint/cmd/corvint", fallback: "whole", own: []string{rel}}
		got := testSourceSlice(root, candidate, map[string]testBaseBlob{rel: {content: []byte(base)}})
		checked++
		if got.Pattern != "" && (got.Pattern == "^$" || !strings.Contains(got.Pattern, frozen)) {
			t.Errorf("a change to %s in %s slices to %q, dropping %s", fn.Name.Name, rel, got.Pattern, frozen)
		}
	}
	if checked == 0 {
		t.Fatal("no Core-verb file has a function with a basic signature; the pin checks nothing")
	}
}

// firstBasicFunction returns the first receiver-less function other than init whose parameters
// and results are all predeclared basic types, or nil.
func firstBasicFunction(file *ast.File) *ast.FuncDecl {
	basic := map[string]bool{"bool": true, "string": true, "int": true, "int64": true, "uint64": true, "byte": true, "rune": true, "float64": true}
	allBasic := func(list *ast.FieldList) bool {
		if list == nil {
			return true
		}
		for _, field := range list.List {
			if id, ok := field.Type.(*ast.Ident); !ok || !basic[id.Name] {
				return false
			}
		}
		return true
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name != "init" && fn.Type.TypeParams == nil && fn.Body != nil && allBasic(fn.Type.Params) && allBasic(fn.Type.Results) {
			return fn
		}
	}
	return nil
}
