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

// V1-5009 criterion 1 (AZU-V0-8910): this repository's own registry, with one
// entry added, plans an edit to itself as DECLARED_RUNTIME_READERS with its
// literal reader selected and no unknown. The actual declaration bytes are
// used, not a fixture copy, so dropping the registry's exact self-entry fails
// here as well as in the CLI plan of any change that declares a new record.
func TestRepositoryRegistryOwnsItsOwnEdit_V1_5009(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(affected.RuntimeReadersPath)))
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	}
	var declaration struct {
		Profile string  `json:"profile"`
		Entries []entry `json:"entries"`
	}
	if err := json.Unmarshal(raw, &declaration); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(declaration.Entries, func(e entry) bool { return e.Path == affected.RuntimeReadersPath }) {
		t.Fatalf("%s does not declare itself by exact entry (AZU-V0-8910)", affected.RuntimeReadersPath)
	}
	declaration.Entries = append(declaration.Entries, entry{Path: "zz-v1-5009/record.json", Reason: "added record"})
	slices.SortFunc(declaration.Entries, func(a, b entry) int {
		if a.Path < b.Path {
			return -1
		}
		return 1
	})
	body, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	const module = "example.test/v15009"
	fsys := fstest.MapFS{
		"go.mod":                    {Data: []byte("module " + module + "\n\ngo 1.27\n")},
		affected.RuntimeReadersPath: {Data: body},
		"planner/planner.go":        {Data: []byte("package planner\n\nconst Path = \".corvint/runtime-readers.json\"\n")},
		"planner/planner_test.go":   {Data: []byte("package planner\n\nimport \"testing\"\n\nfunc TestPath(t *testing.T) { _ = Path }\n")},
		"other/other.go":            {Data: []byte("package other\n")},
		"other/other_test.go":       {Data: []byte("package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {}\n")},
	}
	graph, err := affected.BuildFS(fsys, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{affected.RuntimeReadersPath})
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
	if len(plan.NonSource) != 1 || plan.NonSource[0].Class != affected.ClassDeclaredRuntimeReaders || !slices.Equal(plan.NonSource[0].Paths, []string{affected.RuntimeReadersPath}) {
		t.Fatalf("nonSource=%+v", plan.NonSource)
	}
	selected := func(id string) bool {
		return slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == id })
	}
	if !selected("go:"+module+"/planner") || selected("go:"+module+"/other") {
		t.Fatalf("selected=%+v", plan.Selected)
	}
}
