// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	pathOpenY = "y:dynamic-load:app.y:3 `load(expr)`"
	boundY    = "y:runner-unknown:+3"
	siteY     = "y:runner-unknown:app_test.y"
)

// confinedUnits is a Go-only graph in which every package with tests has a
// declared read scope (AFP-V0-023): an embedding package, its importer, a
// package whose scope names repository files, a package hosting a fixture and
// a package embedding build driver inputs.
func confinedUnits() []Unit {
	return []Unit{
		{ID: "go:cat", Sources: []string{"cat/cat.go"}, Tests: []string{"cat/cat_test.go"}, Embeds: true, EmbedFiles: []string{"cat/data/catalog.json"}, ReadScoped: true, ReadScope: []string{}},
		{ID: "go:user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:cat"}, ReadScoped: true, ReadScope: []string{}},
		{ID: "go:check", Sources: []string{"check/check.go"}, Tests: []string{"check/check_test.go"}, ReadScoped: true,
			ReadScope: []string{".github/workflows/ci.yml", "Makefile", "NOTES.md", "README.md", "fixtures/"}},
		{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}, ReadScoped: true, ReadScope: []string{}},
		{ID: "go:build", Sources: []string{"build/build.go"}, Tests: []string{"build/build_test.go"}, Embeds: true, EmbedFiles: []string{"build/Makefile", "build/rules.mk"}, ReadScoped: true, ReadScope: []string{}},
	}
}

// ownershipGraph builds the confined Go graph, with extra Go units, beside any
// other plugins, and with an optional documentation declaration.
func ownershipGraph(t *testing.T, declaration string, extra []Unit, others ...Language) *Graph {
	t.Helper()
	root := t.TempDir()
	if declaration != "" {
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(DocumentationPathsPath)), []byte(declaration), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	languages := append([]Language{fake{name: "go", units: append(confinedUnits(), extra...)}}, others...)
	graph, err := Build(context.Background(), root, languages...)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func unownedPaths(plan Plan) []string {
	paths := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownUnownedDirtyPath {
			paths = append(paths, unknown.Detail)
		}
	}
	return paths
}

func selectedIDs(plan Plan) []string {
	ids := []string{}
	for _, selection := range plan.Selected {
		ids = append(ids, selection.UnitID)
	}
	return ids
}

// AFP-V0-600: in a graph whose every test process is confined or selected, a
// file a Go package embeds raises no UNOWNED_DIRTY_PATH and selects the
// package and its importers; it stays unowned for the frontier rules.
func TestEmbeddedFileIsAttributedToItsPackage_AFPV0600(t *testing.T) {
	graph := ownershipGraph(t, "", nil)
	plan := Select(graph, []string{"cat/data/catalog.json"})
	if got := unownedPaths(plan); len(got) != 0 {
		t.Fatalf("embedded file raised UNOWNED %v", got)
	}
	if got := fmt.Sprint(selectedIDs(plan)); got != "[go:cat go:user]" {
		t.Fatalf("selected %s, want the embedding package and its importer", got)
	}
}

// AFP-V0-600 (review finding 1): an embed does not make the path Go's source.
// Another plugin whose tests may load it through a computed name (a Jest test
// requiring process.env.CATALOG_PATH) keeps the path UNOWNED, the plan UNKNOWN
// and that plugin's path-open frontier in the plan.
func TestEmbedLeavesForeignReadersOpen_AFPV0600(t *testing.T) {
	plan := Select(ownershipGraph(t, "", nil, bystander()), []string{"cat/data/catalog.json"})
	if got := fmt.Sprint(unownedPaths(plan)); got != "[cat/data/catalog.json]" {
		t.Fatalf("unowned %s, want the embedded path", got)
	}
	if plan.Scope != ScopeUnknown {
		t.Fatalf("scope %s, want UNKNOWN", plan.Scope)
	}
	if got := languageFrontier(plan); fmt.Sprint(got) != fmt.Sprint([]string{pathOpenY, boundY, siteY}) {
		t.Fatalf("frontier %v, want y's path-open entries", got)
	}
	if got := fmt.Sprint(selectedIDs(plan)); !strings.Contains(got, "go:cat") || !strings.Contains(got, "go:user") {
		t.Fatalf("selected %s, want the embedding package and its importer", got)
	}
	// A foreign unit without tests still fails closed.
	quiet := fake{name: "q", units: []Unit{{ID: "q:lib", Sources: []string{"lib.q"}}}}
	if got := fmt.Sprint(unownedPaths(Select(ownershipGraph(t, "", nil, quiet), []string{"cat/data/catalog.json"}))); got != "[cat/data/catalog.json]" {
		t.Fatalf("foreign unit without tests: unowned %s", got)
	}
}

// AFP-V0-601 (review finding 2): a build or CI driver input stays UNOWNED even
// when a Go package embeds it or a read scope names it.
func TestEmbeddedDriverInputStaysUnowned_AFPV0601(t *testing.T) {
	graph := ownershipGraph(t, "", nil)
	for _, dirty := range []string{"build/Makefile", "build/rules.mk", "Makefile", ".github/workflows/ci.yml"} {
		plan := Select(graph, []string{dirty})
		if got := fmt.Sprint(unownedPaths(plan)); got != "["+dirty+"]" || plan.Scope != ScopeUnknown {
			t.Fatalf("%s: unowned %s scope %s, want UNOWNED and UNKNOWN", dirty, got, plan.Scope)
		}
	}
	if got := fmt.Sprint(selectedIDs(Select(graph, []string{"build/Makefile"}))); got != "[go:build]" {
		t.Fatalf("embedded driver input selected %s, want its package still", got)
	}
}

// AFP-V0-601: in a confined graph, data a Go unit's tests read by a rule the
// plan selects it on (a declared read-scope entry or subtree, the subtree of a
// read-scoped package, a Go-hosted testdata fixture) raises no
// UNOWNED_DIRTY_PATH and selects the reader. A declared documentation path
// keeps its AFP-V0-160 class, and a path no rule names stays UNOWNED.
func TestGoReadDataIsAttributedToItsReader_AFPV0601(t *testing.T) {
	const declaration = `{"profile":"corvint-documentation-paths/0","paths":["NOTES.md"]}`
	graph := ownershipGraph(t, declaration, nil)
	cases := []struct {
		dirty, reader string
		attributed    bool
	}{
		{"README.md", "go:check", true},
		{"fixtures/deep/case.json", "go:check", true},
		{"check/golden/out.txt", "go:check", true},
		{"pkg/testdata/cases.json", "go:pkg", true},
		{"loose.txt", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.dirty, func(t *testing.T) {
			plan := Select(graph, []string{tc.dirty})
			if unowned := len(unownedPaths(plan)) != 0; unowned == tc.attributed {
				t.Fatalf("unowned %v, want attributed %v (unknown %v)", unowned, tc.attributed, plan.Unknown)
			}
			if tc.reader != "" && !strings.Contains(fmt.Sprint(selectedIDs(plan)), tc.reader) {
				t.Fatalf("selected %v, want reader %s", selectedIDs(plan), tc.reader)
			}
		})
	}
	plan := Select(graph, []string{"NOTES.md"})
	if len(unownedPaths(plan)) != 0 || len(plan.NonSource) != 1 || plan.NonSource[0].Class != ClassDeclaredDocumentation {
		t.Fatalf("declared documentation lost its class: nonSource %+v unknown %v", plan.NonSource, plan.Unknown)
	}
	// An undeclared host is selected by rule (b), so it may stay unconfined.
	host, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := unownedPaths(Select(host, []string{"pkg/testdata/cases.json"})); len(got) != 0 {
		t.Fatalf("hosted fixture with its unconfined host: unowned %v", got)
	}
	// A read-scoped unit at the repository root attributes nothing by subtree.
	root, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:root", Sources: []string{"main.go"}, Tests: []string{"main_test.go"}, ReadScoped: true, ReadScope: []string{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(unownedPaths(Select(root, []string{"loose.txt"}))); got != "[loose.txt]" {
		t.Fatalf("root read scope attributed a path: unowned %s", got)
	}
}

// AFP-V0-601 (review finding 3): finding one reader proves nothing about the
// rest. An undeclared Go package elsewhere with tests may read any of these
// files through os.Getenv("FIXTURE_PATH"), which no literal or root-locator
// rule resolves, so each stays UNOWNED and the plan UNKNOWN; the named readers
// are still selected.
func TestUnconfinedReaderKeepsReadDataUnowned_AFPV0601(t *testing.T) {
	loose := Unit{ID: "go:loose", Sources: []string{"loose/loose.go"}, Tests: []string{"loose/loose_test.go"}}
	graph := ownershipGraph(t, "", []Unit{loose})
	for _, tc := range []struct{ dirty, reader string }{
		{"pkg/testdata/x.json", "go:pkg"},
		{"README.md", "go:check"},
		{"fixtures/deep/case.json", "go:check"},
		{"cat/data/catalog.json", "go:cat"},
	} {
		plan := Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(unownedPaths(plan)); got != "["+tc.dirty+"]" || plan.Scope != ScopeUnknown {
			t.Fatalf("%s: unowned %s scope %s, want UNOWNED and UNKNOWN", tc.dirty, got, plan.Scope)
		}
		if !strings.Contains(fmt.Sprint(selectedIDs(plan)), tc.reader) {
			t.Fatalf("%s: selected %v, want reader %s", tc.dirty, selectedIDs(plan), tc.reader)
		}
	}
}

// AFP-V0-600: EmbedFiles is part of the observed graph: it changes the digest,
// is validated like every unit path list and survives the graph cache.
func TestEmbedFilesAreDigestedValidatedAndCached_AFPV0600(t *testing.T) {
	unit := Unit{ID: "go:cat", Sources: []string{"cat/cat.go"}, Embeds: true}
	build := func(unit Unit) (*Graph, error) {
		return Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{unit}})
	}
	without, err := build(unit)
	if err != nil {
		t.Fatal(err)
	}
	unit.EmbedFiles = []string{"cat/catalog.json"}
	with, err := build(unit)
	if err != nil {
		t.Fatal(err)
	}
	if with.Digest() == without.Digest() {
		t.Fatal("EmbedFiles must change the graph digest")
	}
	unit.EmbedFiles = []string{"../escape.json"}
	if _, err := build(unit); err == nil {
		t.Fatal("an invalid embed path must fail Build")
	}
	unit.EmbedFiles = []string{"cat/catalog.json"}
	encoded, err := json.Marshal(cachedUnit(unit))
	if err != nil {
		t.Fatal(err)
	}
	var restored cachedUnit
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(Unit(restored), unit) {
		t.Fatalf("cache round trip %+v, want %+v", Unit(restored), unit)
	}
}
