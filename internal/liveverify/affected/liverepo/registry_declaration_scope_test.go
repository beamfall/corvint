// SPDX-License-Identifier: Apache-2.0
package liverepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// The actual repository declaration, not a fixture copy of the expected entry,
// must select the registry when only the CI command declaration changes.
func TestRegistryCICommandsReadScopeSelection(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), affected.ReadScopesPath))
	if err != nil {
		t.Fatal(err)
	}
	var declaration struct {
		Profile  string              `json:"profile"`
		Packages map[string][]string `json:"packages"`
	}
	if err = json.Unmarshal(raw, &declaration); err != nil {
		t.Fatal(err)
	}
	const pkg = "internal/testrunner/registry"
	const changed = ".github/cishards/test-commands.json"
	scope, ok := declaration.Packages[pkg]
	if !ok {
		t.Fatal("registry scope missing")
	}
	if !slices.Contains(scope, changed) {
		t.Fatal("CI declaration read scope missing")
	}
	for _, remove := range []bool{false, true} {
		entries := append([]string{}, scope...)
		if remove {
			entries = slices.DeleteFunc(entries, func(p string) bool { return p == changed })
		}
		declaration.Packages = map[string][]string{pkg: entries}
		body, e := json.Marshal(declaration)
		if e != nil {
			t.Fatal(e)
		}
		fsys := fstest.MapFS{
			"go.mod":                  {Data: []byte("module example.test/registry-scope\n\ngo 1.27\n")},
			pkg + "/registry.go":      {Data: []byte("package registry\n")},
			pkg + "/registry_test.go": {Data: []byte("package registry\nimport (\"testing\";\"os\")\nfunc TestReads(t *testing.T){ p:=os.Getenv(\"DECLARATION\"); _,_=os.ReadFile(p) }\n")},
			changed:                   {Data: []byte("{}\n")}, affected.ReadScopesPath: {Data: body},
		}
		graph, e := affected.BuildFS(fsys, golang.New())
		if e != nil {
			t.Fatal(e)
		}
		plan := affected.Select(graph, []string{changed})
		selected := false
		for _, s := range plan.Selected {
			if s.UnitID == "go:example.test/registry-scope/"+pkg {
				selected = true
				if !remove && s.Witness.Kind != affected.WitnessDeclaredReadScope {
					t.Fatal("wrong witness", s.Witness)
				}
			}
		}
		if selected == remove {
			t.Fatalf("remove=%v selected=%v plan=%+v", remove, selected, plan)
		}
	}
}
