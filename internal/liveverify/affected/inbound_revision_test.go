// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"slices"
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
