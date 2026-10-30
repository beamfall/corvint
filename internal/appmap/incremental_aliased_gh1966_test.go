package appmap

import (
	"context"
	"strings"
	"testing"
)

// AMAP-V0-372 (V1-3059, #1966): with the tests read from an aliased repository whose test root is
// the whole repository (`tests.repo`, `tests.root "."`), a server file added to --root is
// re-derived like a source edit, although the test inputs name "." and kept test files hold bare
// imports, and the map is the full build's. A web module added to --root and any path added to the
// test repository still rebuild in full.
func TestAMAPV0372AliasedTestsAddedPath(t *testing.T) {
	ctx := context.Background()
	app, _, e2e, _ := twoRoots(t, false)
	writeFile(t, app, "appmap.json", strings.Replace(readText(t, app, "appmap.json"), `"root": "e2e"`, `"root": "."`, 1))
	base := commitAll(t, app, "whole-repository test root")
	roots := Roots{Repos: map[string]string{"e2e": e2e}}
	prev, err := BuildRoots(ctx, app, "appmap.json", base, roots)
	if err != nil {
		t.Fatal(err)
	}
	bare := false
	for _, tf := range prev.Files {
		for _, imp := range tf.Imports {
			bare = bare || !strings.HasPrefix(imp.Specifier, ".") && (imp.Status == importUnresolved || imp.Status == importResolved)
		}
	}
	if !bare {
		t.Fatal("fixture holds no bare test import")
	}
	full := func(rev string) *Map {
		t.Helper()
		m, err := BuildRoots(ctx, app, "appmap.json", rev, roots)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"ruby spec and service", map[string]string{"spec/models/item_spec.rb": "describe Item do\nend\n", "app/services/thing.rb": "class Thing\nend\n"}, ""},
		{"web module", map[string]string{"app/lib/extra.js": "export const x = 1;\n"}, RebuildChangedInput},
	} {
		for rel, content := range c.files {
			writeFile(t, app, rel, content)
		}
		head := commitAll(t, app, c.name)
		m, inc, err := BuildFrom(ctx, app, "appmap.json", head, roots, prev)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if inc.Reason != c.want || inc.Reused || (c.want == "") != (strings.Join(inc.Rederived, ",") == "routers,test-joins,flows") {
			t.Errorf("%s: %+v, want reason %q", c.name, inc, c.want)
		}
		if m.Digest != full(head).Digest {
			t.Errorf("%s: digest differs from a full build", c.name)
		}
		gitTest(t, app, "reset", "-q", "--hard", base)
	}
	writeFile(t, e2e, "spec/models/item_spec.rb", "describe Item do\nend\n")
	commitAll(t, e2e, "test repository addition")
	if m, inc, err := BuildFrom(ctx, app, "appmap.json", base, roots, prev); err != nil || inc.Reason != RebuildChangedInput || m.Digest != full(base).Digest {
		t.Fatalf("test repository addition: %+v %v", inc, err)
	}
}
