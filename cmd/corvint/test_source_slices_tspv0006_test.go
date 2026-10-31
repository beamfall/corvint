package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"testing"
)

// TSP-V0-006: the residue of a file without its plain functions ignores
// layout and functions added, removed or edited, and sees every other token;
// a selector qualified by an import counts as no use unless a package-level
// declaration or any other identifier of the file shadows the import name.
func TestTSPV0006PlainFunctionsAndQualifiers(t *testing.T) {
	parse := func(t *testing.T, source string) (*ast.File, *token.FileSet) {
		t.Helper()
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, "f.go", source, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		return file, fileSet
	}
	residue := func(t *testing.T, source string) (string, []string) {
		t.Helper()
		file, fileSet := parse(t, source)
		rest, functions := testSourcePlainFunctions(file, fileSet, []byte(source))
		return rest, slices.Sorted(maps.Keys(functions))
	}
	const base = "package p\n\nvar v = 1\n\n// F does f.\nfunc F() int { return v }\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n"
	baseRest, baseNames := residue(t, base)
	if !slices.Equal(baseNames, []string{"F"}) {
		t.Fatalf("plain functions %v", baseNames)
	}
	for _, tc := range []struct {
		name, source string
		same         bool
	}{
		{"an edited function", "package p\n\nvar v = 1\n\n// F does g.\nfunc F() int {\n\treturn v + 1\n}\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n", true},
		{"an added function", "package p\n\nfunc G() {}\n\nvar v = 1\n\n// F does f.\nfunc F() int { return v }\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n", true},
		{"a removed function", "package p\n\nvar v = 1\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n", true},
		{"blank lines", "package p\nvar v = 1\nfunc (T) M() {}\ntype T struct{}\nfunc init() {}\n", true},
		{"a variable", "package p\n\nvar v = 2\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n", false},
		{"a method", "package p\n\nvar v = 1\n\nfunc (T) M() { _ = 0 }\n\ntype T struct{}\n\nfunc init() {}\n", false},
		{"an init function", "package p\n\nvar v = 1\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() { v++ }\n", false},
		{"a comment", "package p\n\n// v is one.\nvar v = 1\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n", false},
		{"a directive", "//go:build linux\n\npackage p\n\nvar v = 1\n\nfunc (T) M() {}\n\ntype T struct{}\n\nfunc init() {}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rest, _ := residue(t, tc.source); (rest == baseRest) != tc.same {
				t.Fatalf("residue equal %v, want %v", rest == baseRest, tc.same)
			}
		})
	}

	for _, tc := range []struct {
		name, source string
		declared     []string
		want         []string
	}{
		{"imports", "package p\n\nimport (\n\t\"testing\"\n\tx \"example.test/y\"\n\t_ \"embed\"\n\t. \"strings\"\n)\n\nfunc TestA(t *testing.T) { x.F(); _ = ToUpper }\n", nil, []string{"testing", "x"}},
		{"a local shadow", "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { testing := T{}; testing.M() }\n", nil, nil},
		{"a parameter shadow", "package p\n\nimport \"strings\"\n\nfunc F(strings T) { strings.M() }\n", nil, nil},
		{"a package-level shadow", "package p\n\nimport \"example.test/foo\"\n\nfunc F() { foo.M() }\n", []string{"foo"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, _ := parse(t, tc.source)
			declared := make(map[string]bool)
			for _, name := range tc.declared {
				declared[name] = true
			}
			if got := slices.Sorted(maps.Keys(testSourceQualifiers(file, declared))); !slices.Equal(got, tc.want) {
				t.Fatalf("qualifiers %v, want %v", got, tc.want)
			}
		})
	}
	file, _ := parse(t, "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { var v T; v.M(); testing.Short() }\n")
	uses := testSourceDeclOf("f.go", []string{"TestA"}, testSourceQualifiers(file, nil), file.Decls[1]).uses
	if !uses["M"] || !uses["T"] || uses["Short"] {
		t.Fatalf("uses %v: want M and T, not the qualified Short", uses)
	}
}

// TSP-V0-006: a changed function's signature carries only predeclared basic
// value types, and a write the lexical model cannot prove local is named.
func TestTSPV0006ReferenceSignatureAndWrites(t *testing.T) {
	function := func(t *testing.T, source string) *ast.FuncDecl {
		t.Helper()
		file, err := parser.ParseFile(token.NewFileSet(), "f.go", "package p\n\n"+source+"\n", parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		return file.Decls[0].(*ast.FuncDecl)
	}
	for _, tc := range []struct {
		source    string
		declared  map[string]bool
		reference bool
	}{
		{"func F(a, b int, s string) (bool, float64) { return true, 0 }", nil, false},
		{"func F() {}", nil, false},
		{"func F(p *int) {}", nil, true},
		{"func F(s []byte) {}", nil, true},
		{"func F(m map[string]int) {}", nil, true},
		{"func F(c chan int) {}", nil, true},
		{"func F(f func()) {}", nil, true},
		{"func F(v any) {}", nil, true},
		{"func F(v T) {}", nil, true},
		{"func F(v struct{ p *int }) {}", nil, true},
		{"func F(v [2]int) {}", nil, true},
		{"func F(v ...int) {}", nil, true},
		{"func F(v uintptr) {}", nil, true},
		{"func F() *int { return nil }", nil, true},
		{"func F[X any](v int) {}", nil, true},
		{"func F(v int) {}", map[string]bool{"int": true}, true},
	} {
		if got := testSourceReferenceSignature(function(t, tc.source).Type, tc.declared) != ""; got != tc.reference {
			t.Errorf("%s: reference %v, want %v", tc.source, got, tc.reference)
		}
	}
	for _, tc := range []struct{ source, write string }{
		{"func F(n int) int { x := n; x++; x += 2; var y int; y = x; (y) = 1; _ = y; for i := range 3 { x = i }; return x }", ""},
		{"func F() { var s struct{ n int }; s.n = 1 }", "through s.n"},
		{"func F(p *int) { *p = 1 }", "through *p"},
		{"func F() { m := map[int]int{}; m[1] = 2 }", "through m[...]"},
		{"func F() { u.Last++ }", "through u.Last"},
		{"func F() { var a [2]int; for a[0] = range 2 {} }", "through a[...]"},
		{"func F() { var a, b []int; copy(a, b) }", "with copy"},
		{"func F() { m := map[int]int{}; delete(m, 1) }", "with delete"},
		{"func F() { var m map[int]int; clear(m) }", "with clear"},
		{"func F() { go func() { *new(int) = 1 }() }", "through *an expression"},
	} {
		if got := testSourceWrite(function(t, tc.source)); got != tc.write {
			t.Errorf("%s: write %q, want %q", tc.source, got, tc.write)
		}
	}
}
