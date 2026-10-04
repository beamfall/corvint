// SPDX-License-Identifier: AGPL-3.0-or-later
// Private proposed host conformance author. It emits files, not verdicts.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/intake"
)

func fail() { fmt.Fprintln(os.Stderr, "HOSTPAIR_AUTHOR_INPUT"); os.Exit(2) }
func writeFresh(path string, data []byte) {
	if len(data) > 4096 {
		fail()
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fail()
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		fail()
	}
	if f.Close() != nil {
		fail()
	}
}
func main() {
	if len(os.Args) != 3 {
		fail()
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fail()
	}
	raw, err := io.ReadAll(io.LimitReader(f, intake.MaxBytes+1))
	f.Close()
	if err != nil || len(raw) > intake.MaxBytes {
		fail()
	}
	admitted, err := intake.Decode(raw)
	if err != nil || admitted.Intent != "FEATURE" {
		fail()
	}
	sourceClaim := false
	for _, b := range admitted.Behaviours {
		if b.Kind == "ADD" && b.Path == "calc.go" {
			sourceClaim = true
		}
	}
	if !sourceClaim {
		fail()
	}
	data, err := os.ReadFile(filepath.Join(os.Args[2], "calc.go"))
	if err != nil || len(data) > 4096 {
		fail()
	}
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, "calc.go", data, 0)
	if err != nil {
		fail()
	}
	var add *ast.FuncDecl
	for _, d := range parsed.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "Add" {
			if add != nil {
				fail()
			}
			add = fn
		}
	}
	if add == nil || add.Recv != nil || add.Body == nil || len(add.Body.List) != 1 {
		fail()
	}
	params := []string{}
	for _, field := range add.Type.Params.List {
		typ, ok := field.Type.(*ast.Ident)
		if !ok || typ.Name != "int" {
			fail()
		}
		for _, name := range field.Names {
			params = append(params, name.Name)
		}
	}
	if len(params) != 2 || add.Type.Results == nil || len(add.Type.Results.List) != 1 {
		fail()
	}
	result, ok := add.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || result.Name != "int" {
		fail()
	}
	ret, ok := add.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		fail()
	}
	sum, ok := ret.Results[0].(*ast.BinaryExpr)
	if !ok || sum.Op != token.ADD {
		fail()
	}
	left, lok := sum.X.(*ast.Ident)
	right, rok := sum.Y.(*ast.Ident)
	if !lok || !rok || left.Name != params[0] || right.Name != params[1] {
		fail()
	}
	var signature bytes.Buffer
	if printer.Fprint(&signature, set, add.Type) != nil {
		fail()
	}
	rendered := strings.Replace(signature.String(), "func(", "func Add(", 1)
	docs := []byte(fmt.Sprintf("# Add\n\n`%s` returns the sum of its two integer arguments.\n\nExample: `Add(2, 3)` returns `%d`.\n", rendered, 2+3))
	test := []byte(fmt.Sprintf("package tests\n\nimport (\n\"testing\"\nfixture \"example.invalid/pmfixture\"\n)\n\nfunc TestAdd(t *testing.T) {\nif got := fixture.Add(2, 3); got != %d { t.Fatalf(\"Add(2,3) = %%d\", got) }\n}\n", 2+3))
	writeFresh(filepath.Join(os.Args[2], "docs/add.md"), docs)
	writeFresh(filepath.Join(os.Args[2], "tests/add_test.go"), test)
	fmt.Println("HOSTPAIR_AUTHOR_WROTE_DOCS_AND_TEST")
}
