// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// AZU-V0-8900: owning a parser control declaration for audit purposes must
// preserve selection uncertainty and every reader. It grants no directory claim.
func TestAuditControlKeepsSelectionUncertainty_AZU_V0_8900(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, DocumentationPathsPath), []byte(`{"profile":"corvint-documentation-paths/0","paths":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	graph, err := Build(context.Background(), root, fake{name: "g", units: []Unit{{ID: "g:reader", Sources: []string{"reader.g"}, Tests: []string{"reader_test.g"}, PathTokens: []string{DocumentationPathsPath}}}})
	if err != nil {
		t.Fatal(err)
	}
	arbitrary := ".corvint/new-unclaimed.dat"
	if got := graph.OwnershipAudit([]string{DocumentationPathsPath, arbitrary}); !slices.Equal(got, []string{arbitrary}) {
		t.Fatalf("audit claims: %v", got)
	}
	plan := Select(graph, []string{DocumentationPathsPath})
	if !slices.ContainsFunc(plan.Unknown, func(u Unknown) bool { return u.Reason == UnknownUnownedDirtyPath && u.Detail == DocumentationPathsPath }) || !slices.ContainsFunc(plan.Selected, func(s Selection) bool { return s.UnitID == "g:reader" }) {
		t.Fatalf("audit changed selection behavior: %+v", plan)
	}
}

// AZU-V0-8910 (V1-3480): an exact runtime-readers entry for a parser-owned
// control declaration owns its dirty edit: no UNOWNED entry remains, the
// planner unit that names it by literal stays selected, and the ownership
// ratchet runs for a changed selection declaration. A directory claim, a
// non-source claim, and an undeclared `.corvint/` file keep their refusals.
func TestDeclaredControlDeclarationOwnsEdit_AZU_V0_8910(t *testing.T) {
	write := func(t *testing.T, root string, files map[string]string) {
		t.Helper()
		for name, body := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	units := fake{name: "g", units: []Unit{
		{ID: "g:planner", Sources: []string{"planner.g"}, Tests: []string{"planner_test.g"}, PathTokens: []string{DeclaredSelectionPath}},
		{ID: "g:audit", Sources: []string{"audit.g"}, Tests: []string{OwnershipAuditTestPath}},
		{ID: "g:other", Sources: []string{"other.g"}, Tests: []string{"other_test.g"}},
	}}
	root := t.TempDir()
	write(t, root, map[string]string{
		RuntimeReadersPath:    `{"profile":"corvint-runtime-readers/0","entries":[{"path":"` + DeclaredSelectionPath + `","reason":"parsed by the planner through a path literal"}]}`,
		DeclaredSelectionPath: `{"profile":"corvint-test-selection/0"}`,
	})
	graph, err := Build(context.Background(), root, units)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{DeclaredSelectionPath})
	if len(plan.Unknown) != 0 || plan.Scope != ScopeBounded {
		t.Fatalf("declared control declaration left uncertainty: scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
	if !slices.Equal(groupPaths(plan, ClassDeclaredRuntimeReaders), []string{DeclaredSelectionPath}) {
		t.Fatalf("declared control declaration not classified: %+v", plan.NonSource)
	}
	selected := func(id string) bool {
		return slices.ContainsFunc(plan.Selected, func(s Selection) bool { return s.UnitID == id })
	}
	if !selected("g:planner") || !selected("g:audit") || selected("g:other") {
		t.Fatalf("selection: %+v", plan.Selected)
	}
	if got := Select(graph, []string{".corvint/new-unclaimed.dat"}); got.Scope != ScopeUnknown || !slices.ContainsFunc(got.Unknown, func(u Unknown) bool {
		return u.Reason == UnknownUnownedDirtyPath && u.Detail == ".corvint/new-unclaimed.dat"
	}) {
		t.Fatalf("undeclared .corvint file was owned: scope=%s unknown=%+v", got.Scope, got.Unknown)
	}

	refused := map[string]map[string]string{
		"directory entry":  {RuntimeReadersPath: `{"profile":"corvint-runtime-readers/0","entries":[{"path":".corvint/","reason":"too broad"}]}`},
		"non-source claim": {NonSourcePathsPath: `{"profile":"corvint-non-source-paths/0","entries":[{"path":"` + DeclaredSelectionPath + `","reason":"not output"}]}`},
	}
	for name, files := range refused {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, files)
			if _, err := Build(context.Background(), root, units); err == nil {
				t.Fatalf("%s accepted", name)
			}
		})
	}
}
