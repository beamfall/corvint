// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-495: an unowned Ruby/data change can reach tests through a testless
// Go literal reader. Verify the complete chain rather than accepting every
// dependency witness that starts at a unit which happens to exist.
func TestWitnessesResolveFromTestlessReader_AFPV0495(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                "module example.test/reader\n\ngo 1.27\n",
		"reader/reader.go":      "package reader\nvar Paths = []string{\"data/payload.txt\", \"owned/source.go\"}\n",
		"tested/tested.go":      "package tested\nvar Path = \"data/payload.txt\"\n",
		"tested/tested_test.go": "package tested\nimport \"testing\"\nfunc TestRead(*testing.T) {}\n",
		"app/app.go":            "package app\nimport _ \"example.test/reader/reader\"\nimport _ \"example.test/reader/tested\"\n",
		"app/app_test.go":       "package app\nimport \"testing\"\nfunc TestApp(*testing.T) {}\n",
		"other/other.go":        "package other\n",
		"other/other_test.go":   "package other\nimport \"testing\"\nfunc TestOther(*testing.T) {}\n",
		"owned/source.go":       "package owned\n",
		"data/payload.txt":      "retained data\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.BuildPartial(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	const reader, tested, app, other = "go:example.test/reader/reader", "go:example.test/reader/tested", "go:example.test/reader/app", "go:example.test/reader/other"
	found := false
	for _, selection := range affected.Select(graph, []string{"data/payload.txt"}).Selected {
		if !witnessResolves(graph, selection) {
			t.Fatalf("unresolvable witness: %+v", selection)
		}
		if selection.UnitID == app {
			found = true
			if selection.Witness.Kind != affected.WitnessDependency || !slices.Equal(selection.Witness.Via, []string{reader, app}) {
				t.Fatalf("wrong reader chain: %+v", selection.Witness)
			}
		}
	}
	if !found {
		t.Fatal("testless reader's importer not selected")
	}
	for _, forged := range []affected.Selection{
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "data/payload.txt", Via: []string{tested, app}}},
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "owned/source.go", Via: []string{reader, app}}},
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "missing/notnamed.txt", Via: []string{reader, app}}},
		{UnitID: other, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "data/payload.txt", Via: []string{reader, other}}},
	} {
		if witnessResolves(graph, forged) {
			t.Fatalf("forged reader chain resolved: %+v", forged)
		}
	}
}
