// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// writeReachRepository lays out a Go module whose package hosts a testdata directory beside a
// TypeScript tree with one unresolved relative import, one unresolved bare import that no
// package manifest maps and one test file whose runner no configuration fixes.
func writeReachRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "pkg/pkg.go", "package pkg\n")
	write(t, root, "pkg/pkg_test.go", "package pkg\n")
	write(t, root, "src/dep.ts", "export const dep = 1\n")
	write(t, root, "tests/dep.test.ts", "import { test } from \"vitest\"\nimport { dep } from \"../src/dep\"\nimport { gone } from \"./gone\"\n")
	write(t, root, "tests/other.test.ts", "import { dep } from \"../src/dep\"\n")
	return root
}

// AFP-V0-511: the plugin's source-bound reasons bind only paths it owns, so a dirty path no
// plugin owns and no manifest or configuration file the plugin reads raises only its path-open
// entries; a Go-hosted testdata file raises none of its fixture-closed runner entries either; a
// dirty configuration file it reads, such as tsconfig.json, keeps every entry.
func TestFrontierReachHoldsForUnownedPaths_AFPV0511(t *testing.T) {
	root := writeReachRepository(t)
	before, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	alias := FrontierPathAlias + ":tests/dep.test.ts:1 `vitest`"
	relative := FrontierUnresolvedImport + ":tests/dep.test.ts:3 `./gone`"
	runner := FrontierUnknownRunner + ":tests/other.test.ts"
	if fmt.Sprint(before.Frontier) != fmt.Sprint([]string{alias, relative, runner}) {
		t.Fatalf("plugin frontier = %v", before.Frontier)
	}
	// Neither unowned path is read: the observation is the same with them in the tree.
	write(t, root, "Makefile", "all:\n\tnpm test\n")
	write(t, root, "pkg/testdata/cases.json", `[{"name": "a"}]`)
	after, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unowned paths changed the observation:\n%+v\n%+v", before, after)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		want  []string
	}{
		{"Makefile", []string{runner}},
		{"pkg/testdata/cases.json", []string{}},
		{"tsconfig.json", []string{alias, relative, runner}},
		{"src/dep.ts", []string{alias, relative, runner}},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := frontierDetails(plan); fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("dirty %s: frontier %v, want %v", tc.dirty, got, tc.want)
		}
	}
}
