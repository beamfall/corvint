// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestTempJoinedLiteralSelectsNoReader_AFP_V0_9810 plans a docs-only change
// over two packages: one whose test writes a fixture below t.TempDir() under
// a `docs/` literal, and one whose test reads `docs/guide.md` from the
// repository root. Only the reader is selected (PATH_LITERAL_READER); the
// fixture writer, which reads nothing in the checkout, is left out. A third
// package that joins the same literal to the working directory stays a
// reader, since its root is the checkout.
func TestTempJoinedLiteralSelectsNoReader_AFP_V0_9810(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":        "module example.test/m\n\ngo 1.27.0\n",
		"docs/guide.md": "# guide\n",
		"writer/w.go":   "package writer\n",
		"writer/w_test.go": "package writer\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\n" +
			"func TestWrite(t *testing.T) {\n\troot := t.TempDir()\n\tif err := os.MkdirAll(filepath.Join(root, \"docs\"), 0o755); err != nil {\n\t\tt.Fatal(err)\n\t}\n\t_ = os.WriteFile(filepath.Join(root, \"docs\", \"guide.md\"), []byte(\"x\"), 0o644)\n}\n",
		"reader/r.go": "package reader\n",
		"reader/r_test.go": "package reader\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
			"func TestRead(t *testing.T) {\n\t_, _ = os.ReadFile(\"../docs/guide.md\")\n}\n",
		"cwd/c.go": "package cwd\n",
		"cwd/c_test.go": "package cwd\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\n" +
			"func TestCwd(t *testing.T) {\n\troot := \"..\"\n\t_, _ = os.ReadFile(filepath.Join(root, \"docs\", \"guide.md\"))\n}\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/guide.md"})
	selected := map[string]string{}
	for _, selection := range plan.Selected {
		selected[selection.UnitID] = selection.Witness.Kind
	}
	if kind := selected["go:example.test/m/reader"]; kind != affected.WitnessPathLiteralReader {
		t.Errorf("reader selected as %q, want %s; plan %+v", kind, affected.WitnessPathLiteralReader, plan.Selected)
	}
	if kind := selected["go:example.test/m/cwd"]; kind != affected.WitnessPathLiteralReader {
		t.Errorf("cwd joiner selected as %q, want %s", kind, affected.WitnessPathLiteralReader)
	}
	if kind, hit := selected["go:example.test/m/writer"]; hit {
		t.Errorf("the TempDir fixture writer is selected as %s; its docs literal names no repository path", kind)
	}
	unit, _ := graph.Unit("go:example.test/m/writer")
	for _, token := range unit.PathTokens {
		if token == "docs" || token == "guide.md" {
			t.Errorf("writer keeps token %q from the TempDir join: %v", token, unit.PathTokens)
		}
	}
}
