// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// AZU-V0-004, AZU-V0-8904: changing the caller declaration selects old as
// well as new callers. Without base evidence the existing uncertainty stays.
func TestInboundRevisionRetainsOldCallers_AZU_V0_8904(t *testing.T) {
	old := []byte(`{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":{"callers":["old.g"],"reason":"old caller"}}}`)
	head := []byte(`{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":{"callers":["new.g"],"reason":"new caller"}}}`)
	g := fake{name: "g", units: []Unit{
		{ID: "g:old", Sources: []string{"old.g"}, Tests: []string{"old_test.g"}},
		{ID: "g:new", Sources: []string{"new.g"}, Tests: []string{"new_test.g"}},
	}}
	sh := fake{name: "shell", units: []Unit{{ID: "shell:run.sh", Sources: []string{"run.sh"}, Callers: []string{"new.g"}}}}
	graph, err := Build(context.Background(), t.TempDir(), g, sh)
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{ShellForeignCallersPath}
	if _, closed := graph.inboundRevisionPaths(dirty); closed {
		t.Fatal("unobserved base closed")
	}
	graph = graph.WithDriverContent(map[string]DriverRevision{ShellForeignCallersPath: {Base: old, Head: head, BaseExists: true, HeadExists: true}})
	plan := Select(graph, dirty)
	for _, id := range []string{"g:old", "g:new"} {
		if !slices.ContainsFunc(plan.Selected, func(s Selection) bool { return s.UnitID == id }) {
			t.Fatalf("lost caller %s: %+v", id, plan)
		}
	}
	if slices.ContainsFunc(plan.Unknown, func(u Unknown) bool { return u.Detail == FrontierForeignCallersChanged }) {
		t.Fatalf("bound caller change stayed unknown: %+v", plan.Unknown)
	}
	bad := graph.WithDriverContent(map[string]DriverRevision{ShellForeignCallersPath: {Base: old, Head: []byte(`{}`), BaseExists: true, HeadExists: true}})
	if _, closed := bad.inboundRevisionPaths(dirty); closed {
		t.Fatal("invalid head closed")
	}
}

// AZU-V0-8904: both immutable declarations must satisfy the canonical
// shell declaration contract, even when HEAD has no invalid frontier.
func TestInboundRevisionRejectsInvalidSnapshots_AZU_V0_8904(t *testing.T) {
	valid := `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":{"callers":["new.g"],"reason":"valid caller"}}}`
	for _, tc := range []struct{ name, entry string }{
		{"empty lists", `{"callers":[],"mentions":[],"reason":"empty"}`},
		{"unsorted callers", `{"callers":["old.g","new.g"],"reason":"unsorted"}`},
		{"duplicate callers", `{"callers":["new.g","new.g"],"reason":"duplicate"}`},
		{"unsorted mentions", `{"callers":[],"mentions":["old.g","new.g"],"reason":"unsorted"}`},
		{"duplicate mentions", `{"callers":[],"mentions":["new.g","new.g"],"reason":"duplicate"}`},
		{"overlapping lists", `{"callers":["new.g"],"mentions":["new.g"],"reason":"overlap"}`},
		{"self reference", `{"callers":["run.sh"],"reason":"self"}`},
		{"oversized reason", `{"callers":["new.g"],"reason":"` + strings.Repeat("x", 1025) + `"}`},
		{"control reason", `{"callers":["new.g"],"reason":"bad\nreason"}`},
	} {
		for _, side := range []string{"base", "head"} {
			t.Run(tc.name+"/"+side, func(t *testing.T) {
				g := fake{name: "g", units: []Unit{
					{ID: "g:old", Sources: []string{"old.g"}, Tests: []string{"old_test.g"}},
					{ID: "g:new", Sources: []string{"new.g"}, Tests: []string{"new_test.g"}},
				}}
				sh := fake{name: "shell", units: []Unit{{ID: "shell:run.sh", Sources: []string{"run.sh"}, Callers: []string{"new.g"}}}}
				graph, err := Build(context.Background(), t.TempDir(), g, sh)
				if err != nil {
					t.Fatal(err)
				}
				revision := DriverRevision{Base: []byte(valid), Head: []byte(valid), BaseExists: true, HeadExists: true}
				invalid := []byte(`{"profile":"corvint-shell-foreign-callers/0","scripts":{"run.sh":` + tc.entry + `}}`)
				if side == "base" {
					revision.Base = invalid
				} else {
					revision.Head = invalid
				}
				graph = graph.WithDriverContent(map[string]DriverRevision{ShellForeignCallersPath: revision})
				dirty := []string{ShellForeignCallersPath}
				if _, closed := graph.inboundRevisionPaths(dirty); closed {
					t.Fatal("invalid " + side + " discharged caller-change uncertainty")
				}
				plan := Select(graph, dirty)
				if !slices.ContainsFunc(plan.Unknown, func(u Unknown) bool { return u.Detail == FrontierForeignCallersChanged }) {
					t.Fatalf("invalid %s lost changed-declaration frontier: %+v", side, plan)
				}
			})
		}
	}
}
