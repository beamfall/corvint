// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestChangeStatusNarrowedRecord_AFPV011054 (V1-5367): a unit that change
// status drops from the plan is named in plan.changeStatusNarrowed with the
// gate that dropped it and the dirty paths with their status, and is not
// selected; a plan in which status drops nothing carries no record.
func TestChangeStatusNarrowedRecord_AFPV011054(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"script/a.sh":    "true\n",
		"script/b.sh":    "true\n",
		"docs/x.md":      "x\n",
		"lister_test.sh": "test \"$(ls script | wc -l)\" -eq 2\n",
		"walker_test.sh": "find . -name '*.md' | wc -l\n",
		"long_test.sh":   "ls -l script | wc -l\n",
	}
	for i := range 20 {
		files[fmt.Sprintf("notes/n%02d.md", i)] = "n\n"
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	const lister, walker, long = "shell:lister_test.sh", "shell:walker_test.sh", "shell:long_test.sh"
	plan := func(status map[string]affected.ChangeStatus, dirty ...string) affected.Plan {
		return affected.Select(graph.WithChangeStatus(status), dirty)
	}
	selected := func(p affected.Plan) []string {
		var ids []string
		for _, selection := range p.Selected {
			ids = append(ids, selection.UnitID)
		}
		return ids
	}
	// The record is exactly the difference from the plan with every status
	// unknown, and never changes the selection.
	consistent := func(t *testing.T, p affected.Plan) {
		t.Helper()
		without := selected(affected.Select(graph, p.Dirty))
		with := selected(p)
		for _, entry := range p.ChangeStatusNarrowed {
			if slices.Contains(with, entry.UnitID) {
				t.Errorf("%s is recorded as narrowed and selected", entry.UnitID)
			}
			with = append(with, entry.UnitID)
		}
		slices.Sort(with)
		slices.Sort(without)
		if !slices.Equal(with, without) {
			t.Errorf("selected plus narrowed %v, want the unknown-status selection %v", with, without)
		}
	}

	t.Run("an edit below the listed directory", func(t *testing.T) {
		p := plan(map[string]affected.ChangeStatus{"script/a.sh": affected.ChangeModified, "docs/x.md": affected.ChangeModified}, "script/a.sh", "docs/x.md")
		want := []affected.StatusNarrowing{
			{UnitID: lister, Gate: affected.StatusGateNameListing, Paths: []affected.StatusPath{{Path: "script/a.sh", Status: affected.ChangeModified}}, PathCount: 1},
			{UnitID: walker, Gate: affected.StatusGateNameOnlyWalk, Paths: []affected.StatusPath{{Path: "docs/x.md", Status: affected.ChangeModified}, {Path: "script/a.sh", Status: affected.ChangeModified}}, PathCount: 2},
		}
		if !reflect.DeepEqual(p.ChangeStatusNarrowed, want) {
			t.Errorf("changeStatusNarrowed = %+v, want %+v", p.ChangeStatusNarrowed, want)
		}
		ids := selected(p)
		for _, id := range []string{lister, walker} {
			if slices.Contains(ids, id) {
				t.Errorf("%s selected: %v", id, ids)
			}
		}
		if !slices.Contains(ids, long) {
			t.Errorf("the ls -l content read %s is not selected: %v", long, ids)
		}
		consistent(t, p)
		body, err := p.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"changeStatusNarrowed":[{"unitId":"shell:lister_test.sh","gate":"AFP-V0-9962","paths":[{"path":"script/a.sh","status":"modified"}],"pathCount":1}`) {
			t.Errorf("canonical plan lacks the record: %s", body)
		}
	})

	t.Run("the path list is bounded", func(t *testing.T) {
		status := map[string]affected.ChangeStatus{}
		var dirty []string
		for i := range 20 {
			name := fmt.Sprintf("notes/n%02d.md", i)
			status[name] = affected.ChangeModified
			dirty = append(dirty, name)
		}
		p := plan(status, dirty...)
		var found *affected.StatusNarrowing
		for i := range p.ChangeStatusNarrowed {
			if p.ChangeStatusNarrowed[i].UnitID == walker {
				found = &p.ChangeStatusNarrowed[i]
			}
		}
		if found == nil {
			t.Fatalf("walker not recorded: %+v", p.ChangeStatusNarrowed)
		}
		if found.PathCount != 20 || len(found.Paths) != affected.MaxStatusNarrowedPaths || found.Paths[0].Path != "notes/n00.md" {
			t.Errorf("walker record = %+v, want 20 paths counted and the first %d listed", *found, affected.MaxStatusNarrowedPaths)
		}
		consistent(t, p)
	})

	// Nothing is recorded where status drops nothing: an addition, a
	// type change, an unknown status, no status at all, and an edit no
	// narrowed read names.
	for _, row := range []struct {
		name   string
		status map[string]affected.ChangeStatus
		dirty  []string
	}{
		{"an addition", map[string]affected.ChangeStatus{"script/c.sh": affected.ChangeAdded}, []string{"script/c.sh"}},
		{"a type change", map[string]affected.ChangeStatus{"script/a.sh": affected.ChangeTypeChanged}, []string{"script/a.sh"}},
		{"an unknown status", map[string]affected.ChangeStatus{"script/a.sh": affected.ChangeUnknown}, []string{"script/a.sh"}},
		{"no status", nil, []string{"script/a.sh", "docs/x.md"}},
		{"an edit beside an addition", map[string]affected.ChangeStatus{"script/a.sh": affected.ChangeModified, "script/c.sh": affected.ChangeAdded}, []string{"script/a.sh", "script/c.sh"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			p := plan(row.status, row.dirty...)
			if p.ChangeStatusNarrowed != nil {
				t.Errorf("changeStatusNarrowed = %+v, want none", p.ChangeStatusNarrowed)
			}
			consistent(t, p)
			body, err := p.Canonical()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "changeStatusNarrowed") {
				t.Errorf("canonical plan carries an empty record: %s", body)
			}
		})
	}
}
