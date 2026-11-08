// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedTestCounts counts statically runnable entries in the observed test
// files. It never executes discovery or changes selection. -1 is unavailable,
// not a fabricated zero or a fallback to the old test-file count.
func affectedTestCounts(root string, plan affected.Plan) map[string]int {
	counts := make(map[string]int, len(plan.Selected))
	source := affected.DiskSource(root)
	for _, selected := range plan.Selected {
		count := len(selected.Tests)
		if strings.HasPrefix(selected.UnitID, "go:") {
			count = 0
			for _, name := range selected.Tests {
				body, err := source.Read(name)
				if err != nil {
					count = -1
					break
				}
				n := runnableGoTests(name, body)
				if n < 0 {
					count = -1
					break
				}
				count += n
			}
		}
		counts[selected.UnitID] = count
	}
	return counts
}

func runnableGoTests(name string, body []byte) int {
	file, err := parser.ParseFile(token.NewFileSet(), name, body, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return -1
	}
	count := 0
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		name := fn.Name.Name
		if name == "TestMain" && goTestSignature(fn, "M") {
			if fn.Type.TypeParams.NumFields() != 0 {
				return -1
			}
			continue
		}
		kind := ""
		if goTestName(name, "Test") {
			kind = "T"
		} else if goTestName(name, "Fuzz") {
			kind = "F"
		}
		if kind == "" {
			continue
		}
		if !goTestSignature(fn, kind) || fn.Type.TypeParams.NumFields() != 0 {
			return -1
		}
		count++
	}
	for _, example := range doc.Examples(file) {
		if example.Output != "" || example.EmptyOutput {
			count++
		}
	}
	return count
}

// Match cmd/go's discovery name and signature rules, including Unicode and
// TestMain(*testing.T), which is an ordinary test rather than a main hook.
func goTestName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

func goTestSignature(fn *ast.FuncDecl, kind string) bool {
	if fn.Type.Results.NumFields() != 0 || fn.Type.Params.NumFields() != 1 {
		return false
	}
	pointer, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch name := pointer.X.(type) {
	case *ast.Ident:
		return name.Name == kind
	case *ast.SelectorExpr:
		return name.Sel.Name == kind
	}
	return false
}

func projectedTestCount(counts map[string]int, selection affected.Selection) int {
	if counts == nil {
		return len(selection.Tests)
	} // callers projecting synthetic plans
	if count, ok := counts[selection.UnitID]; ok {
		return count
	}
	return -1
}
