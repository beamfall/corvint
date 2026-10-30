package cemcode

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// TestRegisteredListsEveryCode: Registered names exactly the code constants
// declared in cemcode.go, once each, so a code added to the registry reaches
// every package that admits registered codes (V1-3215).
func TestRegisteredListsEveryCode(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "cemcode.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var declared []string
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		for _, spec := range group.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				t.Fatalf("const spec %T is not a value spec", spec)
			}
			for _, value := range values.Values {
				literal, ok := value.(*ast.BasicLit)
				code, err := "", error(nil)
				if ok {
					code, err = strconv.Unquote(literal.Value)
				}
				if !ok || err != nil {
					t.Fatalf("code %v is not a string literal; registered codes must be", values.Names)
				}
				declared = append(declared, code)
			}
		}
	}
	listed := Registered()
	for _, code := range declared {
		if !slices.Contains(listed, code) {
			t.Errorf("code %q is declared but missing from registered in cemcode.go", code)
		}
	}
	slices.Sort(declared)
	sorted := slices.Clone(listed)
	slices.Sort(sorted)
	if !slices.Equal(sorted, slices.Compact(slices.Clone(sorted))) || len(sorted) != len(declared) {
		t.Errorf("registered lists %d codes for %d declared constants, or lists one twice", len(listed), len(declared))
	}
}
