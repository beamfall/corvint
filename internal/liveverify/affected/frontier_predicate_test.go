// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

// deniedSiteFS refuses one path, as an unreadable endpoint does.
type deniedSiteFS struct {
	fstest.MapFS
	denied string
}

func (source deniedSiteFS) Open(name string) (fs.File, error) {
	if name == source.denied {
		return nil, fs.ErrPermission
	}
	return source.MapFS.Open(name)
}

// AFP-V0-9982, AFP-V0-9983 (V1-5365): a frontier site's own file under a
// reader's content-predicate tree is judged by that predicate on the observer
// edge, as a dirty path is. Before the change the site file had no endpoint
// content, so the guard was selected through the site whatever the file held
// (the `typescript:internal/appmap/navigation-events.cjs ->
// tools/flow-guide-check` chain). A matching site file, an unreadable endpoint,
// a plan without sources and a matching dirty edit of the site file still
// select the guard.
func TestFrontierSiteObserverAppliesTreePredicate_AFPV09982(t *testing.T) {
	const site = "y:dynamic-load:tree/loader.y:7 `import`"
	const guard = "go:guard"
	language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{site}, units: []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:loader", Sources: []string{"tree/loader.y"}},
	}}}, directions: map[string]string{"y:dynamic-load": FrontierOutbound}}
	rule := ReadScopeTree{Tree: "tree/", ContentPredicate: "forbidden", Samples: []string{"forbidden"}}
	goPlugin := fake{name: "go", units: []Unit{
		{ID: guard, Sources: []string{"guard/guard.go"}, Tests: []string{"guard/guard_test.go"}, PathTokens: []string{"tree"},
			ReadScoped: true, ReadScope: []string{"tree/"}, ReadScopeTrees: []ReadScopeTree{rule}},
	}}
	graph, err := Build(context.Background(), t.TempDir(), goPlugin, language)
	if err != nil {
		t.Fatal(err)
	}
	if unit, _ := graph.Unit(guard); len(unit.ReadScopeTrees) != 1 {
		t.Fatalf("fixture lost its tree rule: %+v", unit)
	}
	selected := func(graph *Graph, dirty ...string) (Witness, bool) {
		return witnessOf(Select(graph, dirty), guard)
	}
	files := func(loader string) fstest.MapFS {
		return fstest.MapFS{"app/app.y": {Data: []byte("app")}, "tree/loader.y": {Data: []byte(loader)}}
	}
	for _, c := range []struct {
		name       string
		base, head fs.FS
		want       bool
	}{
		{name: "non-matching site file", base: files("ordinary loader"), head: files("ordinary loader")},
		{name: "matching site file", base: files("loads forbidden"), head: files("loads forbidden"), want: true},
		{name: "binary site file", base: files("\x00loader"), head: files("\x00loader"), want: true},
		{name: "unreadable base", base: deniedSiteFS{files("ordinary loader"), "tree/loader.y"}, head: files("ordinary loader"), want: true},
		{name: "unreadable head", base: files("ordinary loader"), head: deniedSiteFS{files("ordinary loader"), "tree/loader.y"}, want: true},
		{name: "missing at head", base: files("ordinary loader"), head: fstest.MapFS{"app/app.y": {Data: []byte("app")}}, want: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			planned := graph.WithReadScopeSources(FSSource(c.base), FSSource(c.head), []string{"app/app.y"})
			witness, hit := selected(planned, "app/app.y")
			if hit != c.want {
				t.Fatalf("guard selected=%v (witness %+v), want %v", hit, witness, c.want)
			}
			if hit && (witness.Kind != WitnessPathLiteralReader || !slices.Contains(witness.Via, "y:loader")) {
				t.Fatalf("guard reached other than through the site: %+v", witness)
			}
			// The site itself is still selected: only the reader edge narrows.
			if _, site := witnessOf(Select(planned, []string{"app/app.y"}), "y:loader"); site {
				t.Fatal("the site unit has no tests and must not be a selection")
			}
		})
	}
	// No sources: nothing is known, so the guard stays selected.
	if _, hit := selected(graph.WithReadScopeSources(nil, nil, []string{"app/app.y"}), "app/app.y"); !hit {
		t.Error("a plan without sources dropped the guard")
	}
	if _, hit := selected(graph, "app/app.y"); !hit {
		t.Error("a plan without read-scope inputs dropped the guard")
	}
	// A dirty edit of the site file keeps the AFP-V0-3300 rule: one that
	// adds matching content selects the guard.
	edited := graph.WithReadScopeSources(FSSource(files("ordinary loader")), FSSource(files("now forbidden")), []string{"tree/loader.y"})
	if _, hit := selected(edited, "tree/loader.y"); !hit {
		t.Error("a matching edit of the site file did not select the guard")
	}
}
