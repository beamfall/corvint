package appmap

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// skVerbOutputs runs the seven source verbs over sources and returns each verb's document with the
// map pin, map digest and repository alias members removed, so a two-root map's answers compare with
// the single-root map's answers over the same files.
func skVerbOutputs(t *testing.T, m *Map, sources []string, o Options) map[string]any {
	t.Helper()
	ctx := context.Background()
	o.Full = true
	out := map[string]any{}
	add := func(name string, raw []byte, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s %v: %v", name, sources, err)
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, raw)
		}
		out[name] = stripKeys(withoutRepo(t, doc), "map_revision", "map_digest", "proposed_repo", "sources")
	}
	raw, err := PageObjects(ctx, m, sources, o)
	add("pageobjects", raw, err)
	raw, err = Steps(ctx, m, sources, o)
	add("steps", raw, err)
	raw, err = Setup(ctx, m, sources, o)
	add("setup", raw, err)
	raw, err = Links(ctx, m, sources, o)
	add("links", raw, err)
	raw, err = InferFlows(ctx, m, sources, nil, o)
	add("infer", raw, err)
	raw, err = How(ctx, m, sources, "book a tee time", nil, nil, nil, o)
	add("how", raw, err)
	r, err := CheckRefs(ctx, m, sources, o)
	if err != nil {
		t.Fatalf("refs %v: %v", sources, err)
	}
	add("refs", r.Document, nil)
	return out
}

func stripKeys(v any, keys ...string) any {
	switch y := v.(type) {
	case map[string]any:
		for _, k := range keys {
			delete(y, k)
		}
		for _, z := range y {
			stripKeys(z, keys...)
		}
	case []any:
		for _, z := range y {
			stripKeys(z, keys...)
		}
	}
	return v
}

// SKH-V0-189: issue #1905. With tests in a second, aliased root the source verbs refused both ways
// an operator could name them: --source e2e (not a directory of --root at the map revision) and
// --root at the E2E checkout (the map revision is not a commit there). --source e2e:DIR with
// --repo e2e=ROOT reads that root at the revision the map pinned, and every verb answers exactly as
// over the single-root map of the same files.
func TestSKHV0189AliasedSources(t *testing.T) {
	ctx := context.Background()
	singleRoot, singleRev := fixtureRepo(t)
	want := skVerbOutputs(t, build(t, singleRoot, singleRev), []string{"e2e"}, Options{Root: singleRoot})

	app, appRev, e2e, e2eRev := twoRoots(t, false)
	m, err := BuildRoots(ctx, app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	// The issue's two refusals: both still refuse, and the first now names the alias remedy.
	_, err = PageObjects(ctx, m, []string{"e2e"}, Options{Root: app})
	if codeOf(err) != "appmap-invalid-query" || !strings.Contains(err.Error(), "pass --source e2e:DIR with --repo e2e=ABSOLUTE_ROOT") {
		t.Fatalf("--source e2e under --root app: %v", err)
	}
	if _, err = PageObjects(ctx, m, []string{"e2e"}, Options{Root: e2e}); codeOf(err) != "appmap-invalid-revision" {
		t.Fatalf("--root e2e: %v", err)
	}
	// An alias whose root is not bound refuses naming it; an alias the map never read refuses.
	if _, err = Steps(ctx, m, []string{"e2e:e2e"}, Options{Root: app}); codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "e2e") {
		t.Fatalf("unbound alias: %v", err)
	}
	if _, err = Steps(ctx, m, []string{"docs:e2e"}, Options{Root: app, Repos: map[string]string{"docs": e2e}}); codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "docs") {
		t.Fatalf("unread alias: %v", err)
	}
	if _, err = Steps(ctx, m, []string{"e2e:nope"}, Options{Root: app, Repos: map[string]string{"e2e": e2e}}); codeOf(err) != "appmap-invalid-query" || !strings.Contains(err.Error(), "root e2e") {
		t.Fatalf("missing aliased dir: %v", err)
	}

	repos := map[string]string{"e2e": e2e}
	got := skVerbOutputs(t, m, []string{"e2e:e2e"}, Options{Root: app, Repos: repos})
	for verb := range want {
		if !reflect.DeepEqual(got[verb], want[verb]) {
			a, _ := json.Marshal(got[verb])
			b, _ := json.Marshal(want[verb])
			t.Fatalf("%s differs from the single-root answer\n got %s\nwant %s", verb, a, b)
		}
	}

	// The aliased root is read at the pinned revision from committed objects: a later commit and a
	// dirty checkout there do not change the answer; a pin that is not a commit there refuses.
	writeFile(t, e2e, "e2e/pages/extra.page.ts", "export class Extra {}\n")
	commitAll(t, e2e, "later")
	writeFile(t, e2e, "e2e/pages/home.page.ts", "// dirty\n")
	if again := skVerbOutputs(t, m, []string{"e2e:e2e"}, Options{Root: app, Repos: repos}); !reflect.DeepEqual(again, got) {
		t.Fatal("aliased sources read the root's HEAD or checkout instead of the map pin")
	}
	if _, err = PageObjects(ctx, m, []string{"e2e:e2e"}, Options{Root: app, Repos: map[string]string{"e2e": t.TempDir()}}); codeOf(err) != "appmap-root-unavailable" {
		t.Fatalf("root without the pinned commit: %v", err)
	}

	// One path read from two roots refuses rather than merging records keyed by path.
	writeFile(t, app, "e2e/pages/home.page.ts", "export class Home {}\n")
	appRev2 := commitAll(t, app, "shadow")
	m2, err := BuildRoots(ctx, app, "appmap.json", appRev2, Roots{Repos: repos, Revisions: map[string]string{"e2e": e2eRev}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PageObjects(ctx, m2, []string{"e2e", "e2e:e2e"}, Options{Root: app, Repos: repos}); codeOf(err) != "appmap-invalid-query" || !strings.Contains(err.Error(), "two roots") {
		t.Fatalf("path in two roots: %v", err)
	}

	// --emit request carries no repository identity for an aliased root, so it refuses.
	if _, err = InferFlowsRequest(ctx, m, []string{"e2e:e2e"}, nil, Options{Root: app, Repos: repos}); codeOf(err) != "appmap-invalid-query" {
		t.Fatalf("emit request over an aliased root: %v", err)
	}
}

// SKH-V0-189: a how answer over tests in an aliased root mixed with views in --root copies the same
// test lines as the single-root answer and proposes the file in the root the copied test came from.
func TestSKHV0189AliasedHowDraft(t *testing.T) {
	ctx := context.Background()
	root, single := skHowFixture(t)
	want, _ := skDraft(t, root, single, "ship an order", nil)

	e2e := t.TempDir()
	copyTree(t, filepath.Join(root, "e2e"), filepath.Join(e2e, "e2e"))
	gitTest(t, e2e, "init", "-q")
	e2eRev := commitAll(t, e2e, "e2e suite")
	gitTest(t, root, "rm", "-rq", "e2e")
	writeFile(t, root, "appmap.json", strings.Replace(railsManifest, `"tests":{`, `"tests":{"repo":"e2e",`, 1))
	appRev := commitAll(t, root, "move tests out")
	m, err := BuildRoots(ctx, root, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil || len(m.Roots) != 1 || m.Roots[0].Revision != e2eRev {
		t.Fatalf("two-root map: %v %+v", err, m)
	}
	raw, err := HowDraft(ctx, m, []string{"e2e:e2e", "app/views"}, "ship an order", nil, nil, nil, Options{Root: root, Repos: map[string]string{"e2e": e2e}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Draft SKDraft `json:"draft"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.Draft
	if got.ProposedRepo != "e2e" || want.ProposedRepo != "" || got.ProposedPath != want.ProposedPath || got.Source != want.Source {
		t.Fatalf("aliased draft %+v, single-root %+v", got, want)
	}
	// Freshness of an aliased anchor stays unknown (AMAP-V0-020), so the aliased answer is at most
	// INCOMPLETE with that gap marked; every copied line is the single-root answer's line.
	if got.Status != HowIncomplete || len(got.Gaps) != 1 || got.Gaps[0].Code != GapStaleEvidence || got.Gaps[0].Detail != "freshness-unknown" {
		t.Fatalf("aliased draft status %s gaps %+v", got.Status, got.Gaps)
	}
	copied := func(d SKDraft) []string {
		out := []string{}
		for _, ev := range d.Evidence {
			if ev.Basis == DraftBasisCopied {
				out = append(out, d.Lines[ev.Line-1]+"|"+ev.Path+"|"+ev.Blob)
			}
		}
		return out
	}
	if a, b := copied(got), copied(want); len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Fatalf("copied lines differ\n got %q\nwant %q", a, b)
	}
}
