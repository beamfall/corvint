// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const treeGuardID = "go:example.test/scopes/guard"

func treeGuardFiles(entry string) fstest.MapFS {
	return fstest.MapFS{
		"go.mod":                    {Data: []byte("module example.test/scopes\n\ngo 1.27\n")},
		"guard/guard_test.go":       {Data: []byte("package guard\nimport (\"testing\"; \"os\")\nfunc TestGuard(t *testing.T) { _, _ = os.ReadDir(\"../cmd\") }\n")},
		affected.ReadScopesPath:     {Data: []byte(`{"profile":"corvint-test-read-scopes/0","packages":{"guard":["cmd/"]}}`)},
		affected.ReadScopeTreesPath: {Data: []byte(`{"profile":"corvint-test-read-scope-trees/0","trees":{"guard":{"cmd/":` + entry + `}}}`)},
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
	const valid = `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden","samples":["import forbidden"]}`
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
		{name: "bad regex", entry: `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"[","samples":["x"]}`, before: "ordinary", after: "ordinary", want: true},
		{name: "missing predicate", entry: `{"reason":"guard/guard_test.go reads cmd/"}`, before: "ordinary", after: "ordinary", want: true},
		{name: "missing samples", entry: `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden"}`, before: "ordinary", after: "ordinary", want: true},
		{name: "uncovered sample", entry: `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden","samples":["different"]}`, before: "ordinary", after: "ordinary", want: true},
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
	denied  string
}

func (source deniedScopeFS) OpenBounded(name string, limit int) (fs.File, error) {
	if name == source.denied {
		*source.reached = true
		return nil, fs.ErrPermission
	}
	return source.MapFS.Open(name)
}

func TestTreePredicateUnreadable_AFP_V0_3300(t *testing.T) {
	graph := treeGuardGraph(t, `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden","samples":["forbidden"]}`)
	files := fstest.MapFS{"cmd/a.txt": {Data: []byte("ordinary")}}
	known := graph.WithReadScopeSources(affected.FSSource(files), affected.FSSource(files), []string{"cmd/a.txt"})
	if treeGuardSelected(known, "cmd/a.txt") {
		t.Fatal("fixture must exclude before injection")
	}
	for _, side := range []string{"base", "head"} {
		t.Run(side, func(t *testing.T) {
			reached := false
			base, head := fs.FS(files), fs.FS(files)
			failure := deniedScopeFS{files, &reached, "cmd/a.txt"}
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
			graph := treeGuardGraph(t, `{"reason":"guard/guard_test.go reads cmd/","listingOnly":true}`)
			graph = graph.WithReadScopeSources(affected.FSSource(c.before), affected.FSSource(c.after), c.dirty)
			if got := treeGuardSelected(graph, c.dirty...); got != c.want {
				t.Fatalf("selected=%v want=%v", got, c.want)
			}
		})
	}
	graph := treeGuardGraph(t, `{"reason":"guard/guard_test.go reads cmd/","listingOnly":true}`)
	if !treeGuardSelected(graph, "cmd/a/main.go") {
		t.Fatal("unknown changes excluded")
	}
}

func TestTreeScopeCacheAndDeclarationIdentity_AFP_V0_3303(t *testing.T) {
	const entry = `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden","samples":["forbidden"]}`
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
	other, _ := restored.Unit(treeGuardID)
	if len(other.ReadScopeTrees) != 1 || other.ReadScopeTrees[0].ContentPredicate != "forbidden" {
		t.Fatalf("lost predicate: %+v", other)
	}
	files := fstest.MapFS{"cmd/a.txt": {Data: []byte("ordinary")}}
	if treeGuardSelected(restored.WithReadScopeSources(affected.FSSource(files), affected.FSSource(files), []string{"cmd/a.txt"}), "cmd/a.txt") {
		t.Fatal("cached predicate ignored")
	}
	prior := map[string][]string{"guard": {"cmd/"}}
	if treeGuardSelected(graph.WithReadScopeBase(prior), affected.ReadScopesPath) {
		t.Fatal("unchanged string-only declaration selected")
	}
	if !treeGuardSelected(graph.WithReadScopeBase(map[string][]string{}), affected.ReadScopesPath) {
		t.Fatal("new string declaration ignored")
	}
	if !treeGuardSelected(graph.WithReadScopeBase(prior), affected.ReadScopeTreesPath) {
		t.Fatal("tree metadata edit ignored")
	}
	// The trees file alone must invalidate a cached predicate, including its
	// removal, malformed contents, legacy values and a later reappearance.
	original := string(treeGuardFiles(entry)[affected.ReadScopeTreesPath].Data)
	for _, body := range []string{
		strings.ReplaceAll(original, "forbidden", "ordinary"),
		"malformed", "", string(treeGuardFiles(`"legacy reason"`)[affected.ReadScopeTreesPath].Data), original,
	} {
		target := filepath.Join(root, affected.ReadScopeTreesPath)
		if body == "" {
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(target, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		changed, outcome, err := affected.BuildCached(context.Background(), root, cache, "tree-scope-test", golang.New())
		if err != nil || outcome.Hit {
			t.Fatalf("metadata reused: %+v %v", outcome, err)
		}
		want := body != original
		if got := treeGuardSelected(changed.WithReadScopeSources(affected.FSSource(files), affected.FSSource(files), []string{"cmd/a.txt"}), "cmd/a.txt"); got != want {
			t.Fatalf("changed metadata selected=%v want=%v: %s", got, want, body)
		}
	}
	if !treeGuardSelected(restored, "cmd/a.txt") {
		t.Fatal("cache retained selection-only revision evidence")
	}
}

func TestOverlappingScopeWitnessNamesBearingEntry_AFP_V0_3303(t *testing.T) {
	fixture := treeGuardFiles(`{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden","samples":["forbidden"]}`)
	fixture[affected.ReadScopesPath].Data = []byte(`{"profile":"corvint-test-read-scopes/0","packages":{"guard":["cmd/","cmd/special/"]}}`)
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

func TestTreeMetadataFailureKeepsPlainScopes_AFP_V0_3300(t *testing.T) {
	const valid = `{"reason":"guard/guard_test.go reads cmd/","contentPredicate":"forbidden","samples":["forbidden"]}`
	content := fstest.MapFS{"cmd/a.txt": {Data: []byte("ordinary")}}
	selected := func(graph *affected.Graph) bool {
		return treeGuardSelected(graph.WithReadScopeSources(affected.FSSource(content), affected.FSSource(content), []string{"cmd/a.txt"}), "cmd/a.txt")
	}
	if selected(treeGuardGraph(t, valid)) {
		t.Fatal("fixture must exclude before injection")
	}
	for _, body := range []string{
		"", "{", `{"profile":"wrong","trees":{}}`,
		`{"profile":"corvint-test-read-scope-trees/0","trees":{}}`,
		`{"profile":"corvint-test-read-scope-trees/0","trees":{"other":{"cmd/":` + valid + `}}}`,
		`{"profile":"corvint-test-read-scope-trees/0","trees":{"guard":{"internal/":` + valid + `}}}`,
		`{"profile":"corvint-test-read-scope-trees/0","trees":{"guard":{"cmd/":null}}}`,
		`{"profile":"corvint-test-read-scope-trees/0","trees":{"guard":{"cmd/":{"reason":"guard read","contentPredicate":".*","samples":["x"]}}}}`,
		`{"profile":"corvint-test-read-scope-trees/0","trees":{"guard":{"cmd/":{"reason":"guard read","unknown":true}}}}`,
	} {
		fixture := treeGuardFiles(valid)
		if body == "" {
			delete(fixture, affected.ReadScopeTreesPath)
		} else {
			fixture[affected.ReadScopeTreesPath].Data = []byte(body)
		}
		graph, err := affected.BuildFS(fixture, golang.New())
		if err != nil {
			t.Fatal(err)
		}
		unit, _ := graph.Unit(treeGuardID)
		if !unit.ReadScoped || !slices.Equal(unit.ReadScope, []string{"cmd/"}) || !selected(graph) {
			t.Fatalf("lost conservative scope: %+v metadata=%s", unit, body)
		}
		if !treeGuardSelected(graph, affected.ReadScopeTreesPath) {
			t.Fatal("declaration edit ignored on metadata failure")
		}
	}
	reached := false
	fixture := treeGuardFiles(valid)
	graph, err := affected.BuildFS(deniedScopeFS{fixture, &reached, affected.ReadScopeTreesPath}, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if !reached {
		t.Fatal("metadata permission failure not reached")
	}
	if !selected(graph) {
		t.Fatal("unreadable metadata excluded guard")
	}
}
