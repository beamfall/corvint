// SPDX-License-Identifier: Apache-2.0
package runnerselection

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin"
)

func nameLiterals(t *testing.T, filename string, source any) []string {
	t.Helper()
	names := map[string]bool{}
	for _, name := range langplugin.Names() {
		names[name] = true
	}
	f, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			s, err := strconv.Unquote(lit.Value)
			if err == nil && names[s] {
				found = append(found, s)
			}
		}
		return true
	})
	return found
}
func TestRSPV0011ZeroLiteralRatchet(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		count++
		if got := nameLiterals(t, e.Name(), nil); len(got) != 0 {
			t.Fatalf("%s names languages/runners: %v; baseline 0", e.Name(), got)
		}
	}
	if count == 0 {
		t.Fatal("no selector files checked")
	}
}
func TestRSPV0011RatchetRejectsRunnerLiteral(t *testing.T) {
	names := langplugin.Names()
	if len(names) == 0 {
		t.Fatal("empty plugin registry")
	}
	source := "package fixture\nconst name = " + strconv.Quote(names[0]) + "\n"
	got := nameLiterals(t, "injected.go", source)
	if len(got) != 1 || got[0] != names[0] {
		t.Fatalf("ratchet missed injected literal: %v", got)
	}
}
