// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const treeGuardID = "go:example.test/scopes/guard"

func treeGuardFiles(entry string) fstest.MapFS {
	return fstest.MapFS{
		"go.mod":                {Data: []byte("module example.test/scopes\n\ngo 1.27\n")},
		"guard/guard_test.go":   {Data: []byte("package guard\nimport (\"testing\"; \"os\")\nfunc TestGuard(t *testing.T) { _, _ = os.ReadDir(\"../cmd\") }\n")},
		affected.ReadScopesPath: {Data: []byte(`{"profile":"corvint-test-read-scopes/0","packages":{"guard":[` + entry + `]}}`)},
	}
}

func treeGuardGraph(t *testing.T, entry string) *affected.Graph {
	t.Helper()
	graph, err := affected.BuildFS(treeGuardFiles(entry), golang.New())
	if err != nil {
		t.Fatal(err)
	}
	unit, ok := graph.Unit(treeGuardID)
	if !ok || !unit.ReadScoped || slices.Contains(graph.Frontier(), golang.FrontierReadScopesInvalid) {
		t.Fatalf("fixture not declared: %+v %v", unit, graph.Frontier())
	}
	return graph
}

func treeGuardSelected(graph *affected.Graph, dirty ...string) bool {
	return slices.ContainsFunc(affected.Select(graph, dirty).Selected, func(s affected.Selection) bool { return s.UnitID == treeGuardID })
}

// The guard also has a path-literal read; it must not bypass the predicate.
func TestTreePredicateEndpointsAndFallback_AFP_V0_3300(t *testing.T) {
	const valid = `{"tree":"cmd/","contentPredicate":"forbidden","samples":["import forbidden"]}`
	for _, c := range []struct {
		name, entry, before, after     string
		baseMissing, headMissing, want bool
	}{
		{name: "head match", entry: valid, before: "ordinary", after: "forbidden", want: true},
		{name: "base match removed", entry: valid, before: "forbidden", after: "ordinary", want: true},
		{name: "neither matches", entry: valid, before: "ordinary", after: "ordinary comment"},
		{name: "binary base", entry: valid, before: "\x00ordinary", after: "ordinary", want: true},
		{name: "binary head", entry: valid, before: "ordinary", after: "\xffordinary", want: true},
		{name: "added", entry: valid, after: "ordinary", baseMissing: true, want: true},
		{name: "deleted", entry: valid, before: "ordinary", headMissing: true, want: true},
		{name: "rename source", entry: valid, before: "ordinary", headMissing: true, want: true},
		{name: "rename destination", entry: valid, after: "ordinary", baseMissing: true, want: true},
		{name: "bad regex", entry: `{"tree":"cmd/","contentPredicate":"[","samples":["x"]}`, before: "ordinary", after: "ordinary", want: true},
		{name: "missing predicate", entry: `{"tree":"cmd/"}`, before: "ordinary", after: "ordinary", want: true},
		{name: "missing samples", entry: `{"tree":"cmd/","contentPredicate":"forbidden"}`, before: "ordinary", after: "ordinary", want: true},
		{name: "uncovered sample", entry: `{"tree":"cmd/","contentPredicate":"forbidden","samples":["different"]}`, before: "ordinary", after: "ordinary", want: true},
		{name: "legacy tree", entry: `"cmd/"`, before: "ordinary", after: "ordinary", want: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			graph := treeGuardGraph(t, c.entry)
			base, head := fstest.MapFS{}, fstest.MapFS{}
			if !c.baseMissing {
				base["cmd/a.txt"] = &fstest.MapFile{Data: []byte(c.before)}
			}
			if !c.headMissing {
				head["cmd/a.txt"] = &fstest.MapFile{Data: []byte(c.after)}
			}
			graph = graph.WithReadScopeSources(affected.FSSource(base), affected.FSSource(head), []string{"cmd/a.txt"})
			if got := treeGuardSelected(graph, "cmd/a.txt"); got != c.want {
				t.Fatalf("selected=%v want=%v", got, c.want)
			}
			if !treeGuardSelected(graph, "guard/guard_test.go") {
				t.Fatal("own tests excluded")
			}
		})
	}
	graph := treeGuardGraph(t, valid)
	if !treeGuardSelected(graph, "cmd/a.txt") {
		t.Fatal("unknown revisions excluded")
	}
}

type deniedScopeFS struct {
	fstest.MapFS
	reached *bool
}

func (source deniedScopeFS) OpenBounded(name string, limit int) (fs.File, error) {
	if name == "cmd/a.txt" {
		*source.reached = true
		return nil, fs.ErrPermission
	}
	return source.MapFS.Open(name)
}

func TestTreePredicateUnreadable_AFP_V0_3300(t *testing.T) {
	graph := treeGuardGraph(t, `{"tree":"cmd/","contentPredicate":"forbidden","samples":["forbidden"]}`)
	files := fstest.MapFS{"cmd/a.txt": {Data: []byte("ordinary")}}
	known := graph.WithReadScopeSources(affected.FSSource(files), affected.FSSource(files), []string{"cmd/a.txt"})
	if treeGuardSelected(known, "cmd/a.txt") {
		t.Fatal("fixture must exclude before injection")
	}
	for _, side := range []string{"base", "head"} {
		t.Run(side, func(t *testing.T) {
			reached := false
			base, head := fs.FS(files), fs.FS(files)
			failure := deniedScopeFS{files, &reached}
			if side == "base" {
				base = failure
			} else {
				head = failure
			}
			injected := graph.WithReadScopeSources(affected.FSSource(base), affected.FSSource(head), []string{"cmd/a.txt"})
			if !reached {
				t.Fatal("permission failure was not reached")
			}
			if !treeGuardSelected(injected, "cmd/a.txt") {
				t.Fatal("unreadable endpoint excluded")
			}
		})
	}
}

func TestListingOnlyTransitions_AFP_V0_3301(t *testing.T) {
	for _, c := range []struct {
		name          string
		before, after fstest.MapFS
		dirty         []string
		want          bool
	}{
		{"content edit", fstest.MapFS{"cmd/a/main.go": {Data: []byte("old")}}, fstest.MapFS{"cmd/a/main.go": {Data: []byte("new")}}, []string{"cmd/a/main.go"}, false},
		{"nested add", fstest.MapFS{"cmd/a/main.go": {}}, fstest.MapFS{"cmd/a/main.go": {}, "cmd/a/new.go": {}}, []string{"cmd/a/new.go"}, false},
		{"child add", fstest.MapFS{"cmd/a/main.go": {}}, fstest.MapFS{"cmd/a/main.go": {}, "cmd/b/main.go": {}}, []string{"cmd/b/main.go"}, true},
		{"child delete", fstest.MapFS{"cmd/a/main.go": {}, "cmd/b/main.go": {}}, fstest.MapFS{"cmd/a/main.go": {}}, []string{"cmd/b/main.go"}, true},
		{"child rename", fstest.MapFS{"cmd/a/main.go": {}}, fstest.MapFS{"cmd/b/main.go": {}}, []string{"cmd/a/main.go", "cmd/b/main.go"}, true},
		{"direct file add", fstest.MapFS{"cmd/a/main.go": {}}, fstest.MapFS{"cmd/a/main.go": {}, "cmd/new.txt": {}}, []string{"cmd/new.txt"}, true},
		{"tree record", fstest.MapFS{"cmd/a/main.go": {}}, fstest.MapFS{"cmd/a/main.go": {}}, []string{"cmd"}, true},
		{"unknown listing", fstest.MapFS{}, fstest.MapFS{"cmd/a/main.go": {}}, []string{"cmd/a/main.go"}, true},
		{"unknown entry kind", fstest.MapFS{"cmd/a": {Mode: fs.ModeSymlink}}, fstest.MapFS{"cmd/a": {Mode: fs.ModeSymlink}}, []string{"cmd/a"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			graph := treeGuardGraph(t, `{"tree":"cmd/","listingOnly":true}`)
			graph = graph.WithReadScopeSources(affected.FSSource(c.before), affected.FSSource(c.after), c.dirty)
			if got := treeGuardSelected(graph, c.dirty...); got != c.want {
				t.Fatalf("selected=%v want=%v", got, c.want)
			}
		})
	}
	graph := treeGuardGraph(t, `{"tree":"cmd/","listingOnly":true}`)
	if !treeGuardSelected(graph, "cmd/a/main.go") {
		t.Fatal("unknown changes excluded")
	}
}

func TestTreeScopeCacheAndDeclarationIdentity_AFP_V0_3303(t *testing.T) {
	const entry = `{"tree":"cmd/","contentPredicate":"forbidden","samples":["forbidden"]}`
	root := t.TempDir()
	for name, file := range treeGuardFiles(entry) {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, file.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	oldWindow := affected.GraphCacheRacyWindow
	affected.GraphCacheRacyWindow = 0
	t.Cleanup(func() { affected.GraphCacheRacyWindow = oldWindow })
	cache := t.TempDir()
	graph, _, err := affected.BuildCached(context.Background(), root, cache, "tree-scope-test", golang.New())
	if err != nil {
		t.Fatal(err)
	}
	restored, outcome, err := affected.BuildCached(context.Background(), root, cache, "tree-scope-test", golang.New())
	if err != nil || !outcome.Hit {
		t.Fatalf("cache restore: %+v %v", outcome, err)
	}
	unit, _ := graph.Unit(treeGuardID)
	other, _ := restored.Unit(treeGuardID)
	if len(other.ReadScopeTrees) != 1 || other.ReadScopeTrees[0].ContentPredicate != "forbidden" {
		t.Fatalf("lost predicate: %+v", other)
	}
	files := fstest.MapFS{"cmd/a.txt": {Data: []byte("ordinary")}}
	if treeGuardSelected(restored.WithReadScopeSources(affected.FSSource(files), affected.FSSource(files), []string{"cmd/a.txt"}), "cmd/a.txt") {
		t.Fatal("cached predicate ignored")
	}
	prior := affected.ReadScopeIdentities(unit.ReadScope, unit.ReadScopeTrees)
	if treeGuardSelected(graph.WithReadScopeBase(map[string][]string{"guard": prior}), affected.ReadScopesPath) {
		t.Fatal("unchanged declaration selected")
	}
	if !treeGuardSelected(graph.WithReadScopeBase(map[string][]string{"guard": {"cmd/"}}), affected.ReadScopesPath) {
		t.Fatal("new predicate identity ignored")
	}
	plain := treeGuardGraph(t, `"cmd/"`)
	if !treeGuardSelected(plain.WithReadScopeBase(map[string][]string{"guard": prior}), affected.ReadScopesPath) {
		t.Fatal("removed predicate identity ignored")
	}
	if !treeGuardSelected(restored, "cmd/a.txt") {
		t.Fatal("cache retained selection-only revision evidence")
	}
}

func TestOverlappingScopeWitnessNamesBearingEntry_AFP_V0_3303(t *testing.T) {
	fixture := treeGuardFiles(`{"tree":"cmd/","contentPredicate":"forbidden","samples":["forbidden"]},"cmd/special/"`)
	// Exercise DECLARED_READ_SCOPE itself, without the earlier literal-reader witness.
	fixture["guard/guard_test.go"].Data = []byte("package guard\nimport \"testing\"\nfunc TestGuard(t *testing.T) {}\n")
	graph, err := affected.BuildFS(fixture, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"cmd/special/a.txt": {Data: []byte("ordinary")}}
	graph = graph.WithReadScopeSources(affected.FSSource(files), affected.FSSource(files), []string{"cmd/special/a.txt"})
	for _, selected := range affected.Select(graph, []string{"cmd/special/a.txt"}).Selected {
		if selected.UnitID == treeGuardID {
			if selected.Witness.ReadScope == nil || selected.Witness.ReadScope.Entry != "cmd/special/" {
				t.Fatalf("wrong bearing entry: %+v", selected.Witness)
			}
			return
		}
	}
	t.Fatal("plain overlapping scope excluded")
}
