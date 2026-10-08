package appmap

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// copyTree writes the files under src into dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		writeFile(t, dst, filepath.ToSlash(rel), string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// twoRoots splits the fixture into a product repository (routers, flows) and an E2E repository
// (tests and their package.json). The manifest names the E2E root as "e2e" and is committed in the
// E2E repository when manifestInE2E, else in the product repository.
func twoRoots(t *testing.T, manifestInE2E bool) (app, appRev, e2e, e2eRev string) {
	t.Helper()
	app, _ = fixtureRepo(t)
	raw, err := os.ReadFile(filepath.Join(app, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := strings.Replace(string(raw), `"tests": {`, `"tests": {"repo": "e2e",`, 1)
	gitTest(t, app, "rm", "-rq", "e2e", "appmap.json")
	e2e = t.TempDir()
	copyTree(t, "testdata/fixture/e2e", filepath.Join(e2e, "e2e"))
	pkg, _ := os.ReadFile("testdata/fixture/package.json")
	writeFile(t, e2e, "package.json", string(pkg))
	if manifestInE2E {
		writeFile(t, e2e, "appmap.json", manifest)
	} else {
		writeFile(t, app, "appmap.json", manifest)
	}
	gitTest(t, e2e, "init", "-q")
	return app, commitAll(t, app, "move tests out"), e2e, commitAll(t, e2e, "e2e suite")
}

// withoutRepo re-encodes v with every "repo" member removed, so a two-root map's join can be
// compared with the single-root map of the same files.
func withoutRepo(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	var strip func(any)
	strip = func(x any) {
		switch y := x.(type) {
		case map[string]any:
			delete(y, "repo")
			for _, z := range y {
				strip(z)
			}
		case []any:
			for _, z := range y {
				strip(z)
			}
		}
	}
	strip(out)
	return out
}

// AMAP-V0-017 AMAP-V0-018: routers in one repository and tests in a second, aliased one join
// exactly as when both live in one repository; the map pins the second root's HEAD and names it on
// every test-side anchor and unknown.
func TestAMAPV0017TwoRootJoin(t *testing.T) {
	singleRoot, singleRev := fixtureRepo(t)
	single := build(t, singleRoot, singleRev)
	app, appRev, e2e, e2eRev := twoRoots(t, false)
	m, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	if m.Revision != appRev || !reflect.DeepEqual(m.Roots, []RootPin{{Repo: "e2e", Revision: e2eRev, Inputs: []string{"tests"}}}) {
		t.Fatalf("root pins: %s %+v", m.Revision, m.Roots)
	}
	if m.Manifest.Repo != "" {
		t.Fatalf("manifest read from %q", m.Manifest.Repo)
	}
	for _, part := range []struct {
		name      string
		got, want any
	}{{"screens", m.Screens, single.Screens}, {"edges", m.Edges, single.Edges}, {"flows", m.Flows, single.Flows},
		{"files", m.Files, single.Files}, {"unknowns", m.Unknowns, single.Unknowns}} {
		if !reflect.DeepEqual(withoutRepo(t, part.got), withoutRepo(t, part.want)) {
			t.Errorf("%s differ from the single-root join", part.name)
		}
	}
	if len(m.Files) == 0 {
		t.Fatal("no test files joined")
	}
	for _, f := range m.Files {
		if f.Anchor.Repo != "e2e" {
			t.Errorf("%s anchor repo %q", f.Path, f.Anchor.Repo)
		}
		for _, me := range f.Methods {
			if me.Anchor.Repo != "e2e" {
				t.Errorf("%s anchor repo %q", me.ID, me.Anchor.Repo)
			}
		}
	}
	for _, s := range m.Screens {
		if s.Anchor.Repo != "" {
			t.Errorf("router anchor %s read from %q", s.ID, s.Anchor.Repo)
		}
	}
	for _, e := range m.Edges {
		if (e.Basis == "test-sequence") != (e.Anchor.Repo == "e2e") {
			t.Errorf("edge %s->%s (%s) anchor repo %q", e.From, e.To, e.Basis, e.Anchor.Repo)
		}
	}
	for _, u := range m.Unknowns {
		if want := map[string]bool{"file": true, "import": true, "selector": true, "page-object": true, "test-join": true}[u.Kind]; want != (u.Repo == "e2e") {
			t.Errorf("unknown %s %s repo %q", u.Kind, u.Ref, u.Repo)
		}
	}
	first, _ := Encode(m)
	again, _ := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if second, _ := Encode(again); string(first) != string(second) {
		t.Fatal("two-root builds differ")
	}
	file := filepath.Join(t.TempDir(), "map.json")
	if err = os.WriteFile(file, first, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadMap(file); err != nil {
		t.Fatalf("two-root map does not load: %v", err)
	}

	// The manifest may itself be committed in the aliased root.
	app, appRev, e2e, e2eRev = twoRoots(t, true)
	m, err = BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}, ManifestRepo: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Manifest.Repo != "e2e" || !reflect.DeepEqual(m.Roots, []RootPin{{Repo: "e2e", Revision: e2eRev, Inputs: []string{"manifest", "tests"}}}) {
		t.Fatalf("manifest %q pins %+v", m.Manifest.Repo, m.Roots)
	}
	if _, err = BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}}); codeOf(err) != "appmap-invalid-manifest" {
		t.Fatalf("manifest absent from --root read: %v", err)
	}
}

// AMAP-V0-017: an undeclared alias, a root that is not the top of a Git worktree, a relative or
// missing path, --root itself under an alias, or an invalid alias refuses; nothing is joined.
func TestAMAPV0017RootUnavailable(t *testing.T) {
	app, appRev, e2e, _ := twoRoots(t, false)
	plain := t.TempDir()
	for name, repos := range map[string]map[string]string{
		"undeclared":   nil,
		"other alias":  {"tests": e2e},
		"not git":      {"e2e": plain},
		"subdirectory": {"e2e": filepath.Join(e2e, "e2e")},
		"relative":     {"e2e": "e2e"},
		"missing":      {"e2e": filepath.Join(plain, "absent")},
		"same as root": {"e2e": app},
	} {
		m, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: repos})
		if codeOf(err) != "appmap-root-unavailable" || m != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{ManifestRepo: "e2e"}); codeOf(err) != "appmap-root-unavailable" {
		t.Errorf("undeclared manifest root: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(app, "appmap.json"))
	writeFile(t, app, "appmap.json", strings.Replace(string(raw), `"repo": "e2e"`, `"repo": "E2E"`, 1))
	rev := commitAll(t, app, "bad alias")
	if _, err := BuildRoots(context.Background(), app, "appmap.json", rev, Roots{Repos: map[string]string{"e2e": e2e}}); codeOf(err) != "appmap-invalid-manifest" {
		t.Errorf("invalid alias: %v", err)
	}
}

// AMAP-V0-019: the aliased root is read at HEAD, so a staged, unstaged or untracked change under
// what the map reads from it refuses instead of joining a tree that differs from the checkout; a
// change elsewhere in that root does not.
func TestAMAPV0019DirtySecondRoot(t *testing.T) {
	app, appRev, e2e, _ := twoRoots(t, false)
	roots := Roots{Repos: map[string]string{"e2e": e2e}}
	writeFile(t, e2e, "README.md", "outside the tests root\n")
	if _, err := BuildRoots(context.Background(), app, "appmap.json", appRev, roots); err != nil {
		t.Fatalf("change outside tests.root refused: %v", err)
	}
	page := filepath.Join(e2e, "e2e/pages/home.page.ts")
	data, _ := os.ReadFile(page)
	for name, dirty := range map[string]func(){
		"unstaged": func() { writeFile(t, e2e, "e2e/pages/home.page.ts", string(data)+"// edit\n") },
		"staged": func() {
			writeFile(t, e2e, "e2e/pages/home.page.ts", string(data)+"// edit\n")
			gitTest(t, e2e, "add", "e2e/pages/home.page.ts")
		},
		"untracked": func() { writeFile(t, e2e, "e2e/specs/new.spec.ts", "export {};\n") },
	} {
		dirty()
		if m, err := BuildRoots(context.Background(), app, "appmap.json", appRev, roots); codeOf(err) != "appmap-root-dirty" || m != nil {
			t.Errorf("%s: %v", name, err)
		}
		gitTest(t, e2e, "reset", "-q", "--hard")
		gitTest(t, e2e, "clean", "-qfd", "e2e")
	}
	if _, err := BuildRoots(context.Background(), app, "appmap.json", appRev, roots); err != nil {
		t.Fatalf("clean root refused: %v", err)
	}
}

// AMAP-V0-018: a single-root manifest compiles to the same bytes with or without declared roots,
// and its map carries no roots or repo member.
func TestAMAPV0018SingleRootUnchanged(t *testing.T) {
	root, rev := fixtureRepo(t)
	want, err := Encode(build(t, root, rev))
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildRoots(context.Background(), root, "appmap.json", rev, Roots{Repos: map[string]string{"e2e": t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Encode(m)
	if string(got) != string(want) {
		t.Fatal("an unused --repo changed a single-root map")
	}
	if strings.Contains(string(got), `"repo"`) || strings.Contains(string(got), `"roots"`) {
		t.Fatal("single-root map carries root members")
	}
}

// AMAP-V0-020: projections read only --root, so an anchor from the aliased root reads UNKNOWN (never
// another repository's same path) and is printed with its repo; router anchors stay FRESH, and the
// scaffold names the root its proposed file belongs in.
func TestAMAPV0020ProjectionsReadAliasedAnchorsUnknown(t *testing.T) {
	app, appRev, e2e, _ := twoRoots(t, false)
	m, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	// The same test path in --root must not make an aliased anchor FRESH.
	copyTree(t, filepath.Join(e2e, "e2e"), filepath.Join(app, "e2e"))
	commitAll(t, app, "same paths in the product repo")
	doc := screenDoc(t, m, "app.clubs.teesheets", Options{Root: app, Full: true})
	items, _ := doc["page_object_methods"].([]any)
	if len(items) == 0 {
		t.Fatal("no page-object methods projected")
	}
	for _, it := range items {
		anchor := it.(map[string]any)["anchor"].(map[string]any)
		if anchor["freshness"] != FreshUnknown || anchor["repo"] != "e2e" {
			t.Errorf("aliased method anchor %v", anchor)
		}
	}
	if got := doc["screen"].(map[string]any)["anchor"].(map[string]any); got["freshness"] != Fresh || got["repo"] != nil {
		t.Errorf("router anchor %v", got)
	}
	raw, err := ProjectScaffold(context.Background(), m, "book-tee-time", Options{Root: app})
	if err != nil {
		t.Fatal(err)
	}
	if sc := decode(t, raw); sc["proposed_repo"] != "e2e" || sc["proposed_path"] != "e2e/specs/book-tee-time.spec.ts" {
		t.Errorf("scaffold proposed %v in %v", sc["proposed_path"], sc["proposed_repo"])
	}
	// find names the root of every test-side reference.
	if raw, err = ProjectFind(context.Background(), m, "teesheet", Options{Root: app, Full: true}); err != nil {
		t.Fatal(err)
	}
	found := decode(t, raw)
	for _, section := range []string{"files", "methods"} {
		items, _ := found[section].([]any)
		if len(items) == 0 {
			t.Fatalf("find %s: none", section)
		}
		for _, it := range items {
			if it.(map[string]any)["repo"] != "e2e" {
				t.Errorf("find %s item %v", section, it)
			}
		}
	}
	// The planner reads aliased methods and specs UNKNOWN and names their root (AMSP-V0).
	if raw, err = Plan(context.Background(), []*Map{m}, []string{"book a tee time"}, PlanOptions{Options: Options{Root: app}}); err != nil {
		t.Fatal(err)
	}
	methods, specs := 0, 0
	for _, st := range planSteps(t, decode(t, raw)) {
		if sp, ok := st["spec"].(map[string]any); ok {
			specs++
			if sp["repo"] != "e2e" || sp["freshness"] != FreshUnknown {
				t.Errorf("plan spec %v", sp)
			}
		}
		for _, a := range actions(st) {
			for _, me := range a["methods"].([]any) {
				methods++
				if v := me.(map[string]any); v["repo"] != "e2e" || v["freshness"] != FreshUnknown {
					t.Errorf("plan method %v", v)
				}
			}
		}
	}
	if methods == 0 || specs == 0 {
		t.Fatalf("plan cites %d methods and %d specs", methods, specs)
	}
}

// AMAP-V0-017: a root whose path ends in a space is a valid MCPV0-001 root and is not trimmed.
func TestAMAPV0017RootPathKeepsTrailingSpace(t *testing.T) {
	app, appRev, e2e, _ := twoRoots(t, false)
	spaced := filepath.Join(t.TempDir(), "e2e ")
	if err := os.Rename(e2e, spaced); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": spaced}}); err != nil {
		t.Fatalf("root with a trailing space refused: %v", err)
	}
}
