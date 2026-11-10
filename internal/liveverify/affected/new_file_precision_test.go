// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TSP-V0-1062: a newly added test never changes its package's importers;
// explicit source-reading consumers remain selected with the exact declaration.
func TestNewTestKeepsNamedDeclaredConsumers_TSPV01062(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                "module example.test/m\n\ngo 1.27\n",
		"a/a.go":                "package a\nfunc A() {}\n",
		"a/new_test.go":         "package a\nimport \"testing\"\nfunc TestNew(t *testing.T) {}\n",
		"user/user.go":          "package user\nimport \"example.test/m/a\"\nfunc Use(){a.A()}\n",
		"user/user_test.go":     "package user\nimport \"testing\"\nfunc TestUse(t *testing.T) {}\n",
		"reader/reader.go":      "package reader\n",
		"reader/reader_test.go": "package reader\nimport \"testing\"\nfunc TestRead(t *testing.T) {}\n",
		affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"reader":["a/"]}}`,
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"a/new_test.go"})
	got := map[string]affected.Witness{}
	for _, s := range plan.Selected {
		got[s.UnitID] = s.Witness
	}
	if len(got) != 2 || got["go:example.test/m/a"].Kind != affected.WitnessDirectTest {
		t.Fatalf("new test selected %+v", got)
	}
	w := got["go:example.test/m/reader"]
	if w.Kind != affected.WitnessDeclaredReadScope || w.ReadScope == nil || w.ReadScope.Declaration != affected.ReadScopesPath || w.ReadScope.Entry != "a/" {
		t.Fatalf("reader attribution: %+v", w)
	}
	// Production changes still reach the importer.
	if _, ok := selectedUnits(graph, "example.test/m", "a/a.go")["user"]; !ok {
		t.Fatal("source importer omitted")
	}
}

// selectedUnits maps each unit the plan for the dirty paths selects to its
// witness kind, with Go package ids shortened to their repository directory.
func selectedUnits(graph *affected.Graph, module string, dirty ...string) map[string]string {
	units := map[string]string{}
	for _, selection := range affected.Select(graph, dirty).Selected {
		units[strings.TrimPrefix(selection.UnitID, "go:"+module+"/")] = selection.Witness.Kind
	}
	return units
}
