package changee2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// twoRepoFixture splits the CCE fixture into an application repository (routers, flows, the
// manifest and `.corvint/`) and a separate hosted test repository holding e2e/, the layout of
// CCE-V0-130, with the block declarations beside the suite. The manifest names the test repository "e2e". It returns the application root at
// base, the test root and its commit.
func twoRepoFixture(t *testing.T) (app, base, e2e, e2eRev string) {
	t.Helper()
	app, _ = fixtureRepo(t)
	raw, err := os.ReadFile(filepath.Join(app, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := strings.Replace(string(raw), `"tests": {`, `"tests": {"repo": "e2e",`, 1)
	e2e = t.TempDir()
	copyTree(t, filepath.Join(app, "e2e"), filepath.Join(e2e, "e2e"))
	copyTree(t, filepath.Join(app, ".corvint"), filepath.Join(e2e, ".corvint"))
	gitTest(t, app, "rm", "-rq", "e2e", DeclarationsPath)
	base = commitFiles(t, app, map[string]*string{"appmap.json": &manifest}, "move tests out")
	gitTest(t, e2e, "init", "-q", "-b", "main")
	return app, base, e2e, commitAll(t, e2e, "e2e suite")
}

// CCE-V0-130, CCE-V0-131: catalog and obligations over an application repository and a separate
// test repository. The diff is --root's; page objects and spans read from the test root carry its
// alias; every report records the test root's pin; the two-repo catalog admits the same blocks as
// the single-repo one; an unbound application path still reads UNKNOWN, never NONE.
func TestCCEV0130TwoRepoCatalogAndObligations(t *testing.T) {
	single, singleBase := fixtureRepo(t)
	one, err := BuildCatalog(bg, CatalogOptions{Root: single, Rev: singleBase, Manifest: "appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	app, base, e2e, e2eRev := twoRepoFixture(t)
	roots := appmap.Roots{Repos: map[string]string{"e2e": e2e}}
	if _, err := BuildCatalog(bg, CatalogOptions{Root: app, Rev: base, Manifest: "appmap.json"}); refusalCode(err) != "e2e-map-unavailable" ||
		!strings.Contains(err.Error(), "--repo e2e=") {
		t.Fatalf("undeclared test root: %v", err)
	}
	c, err := BuildCatalog(bg, CatalogOptions{Root: app, Rev: base, Manifest: "appmap.json", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Roots) != 1 || c.Roots[0].Repo != "e2e" || c.Roots[0].Revision != e2eRev || c.Roots[0].Stated {
		t.Fatalf("catalog pins %+v", c.Roots)
	}
	if c.Declarations != "DECLARED" {
		t.Fatalf("declarations beside the suite were not read from the test root: %s", c.Declarations)
	}
	ids := func(c *Catalog) string {
		var out []string
		for _, b := range c.Blocks {
			out = append(out, b.ID)
		}
		return strings.Join(out, ",")
	}
	if ids(c) != ids(one) || len(c.Blocks) == 0 {
		t.Fatalf("two-repo blocks %s\nsingle-repo blocks %s", ids(c), ids(one))
	}
	pageObjects := 0
	for _, b := range c.Blocks {
		if b.Kind == KindPageObject || b.Span.Path == DeclarationsPath {
			pageObjects++
			if b.Span.Repo != "e2e" {
				t.Fatalf("block read from the test root without its alias: %+v", b)
			}
		} else if b.Span.Repo != "" {
			t.Fatalf("application block carries an alias: %+v", b)
		}
	}
	if pageObjects == 0 {
		t.Fatal("no page-object block from the test root")
	}

	head := commitBehaviourChange(t, app)
	o, err := BuildObligations(bg, DiffOptions{Root: app, Base: base, Head: head, Manifest: "appmap.json", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if o.State != Reached || len(o.Obligations) != 1 || o.Obligations[0].ID != "obligation:book-tee-time/booked" ||
		len(o.Roots) != 1 || o.Roots[0].Revision != e2eRev {
		t.Fatalf("%s", o.Encoded())
	}
	backend := commitFiles(t, app, map[string]*string{"server/handler.go": text("package server\n")}, "backend")
	o, err = BuildObligations(bg, DiffOptions{Root: app, Base: head, Head: backend, Manifest: "appmap.json", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if o.State != Unknown || len(o.Unknown) != 1 || o.Unknown[0].Span.Path != "server/handler.go" || o.Unknown[0].Span.Repo != "" ||
		o.Unknown[0].Reason != ReasonUnplacedPath {
		t.Fatalf("unbound application path: %s", o.Encoded())
	}
}

// CCE-V0-132: a stated test-root revision is read from committed objects, recorded as stated, and
// keeps the catalog digest when the test root moves on or is dirty.
func TestCCEV0132StatedTestRootRevision(t *testing.T) {
	app, base, e2e, e2eRev := twoRepoFixture(t)
	roots := appmap.Roots{Repos: map[string]string{"e2e": e2e}, Revisions: map[string]string{"e2e": e2eRev}}
	first, err := BuildCatalog(bg, CatalogOptions{Root: app, Rev: base, Manifest: "appmap.json", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Roots) != 1 || !first.Roots[0].Stated || first.Roots[0].Revision != e2eRev {
		t.Fatalf("pins %+v", first.Roots)
	}
	commitFiles(t, e2e, map[string]*string{"e2e/specs/later.spec.ts": text("export {};\n")}, "later")
	if err := os.WriteFile(filepath.Join(e2e, "e2e/pages/club.page.ts"), []byte("// dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := BuildCatalog(bg, CatalogOptions{Root: app, Rev: base, Manifest: "appmap.json", Roots: roots})
	if err != nil || again.Digest != first.Digest {
		t.Fatalf("stated revision moved: %v", err)
	}
	if _, err := BuildCatalog(bg, CatalogOptions{Root: app, Rev: base, Manifest: "appmap.json", Roots: appmap.Roots{Repos: roots.Repos}}); refusalCode(err) != "e2e-map-unavailable" ||
		!strings.Contains(err.Error(), "appmap-root-dirty") && !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("HEAD read of a dirty test root: %v", err)
	}
	// An acceptance ledger read from the test root has no base side in --root's diff, so each
	// accepted proposal is flagged as accepted by the change (CCE-V0-044).
	aliased := &Catalog{tests: testsRoot{dir: e2e, rev: e2eRev, repo: "e2e"}, Blocks: []Block{{ID: "path:e2e/a.spec.ts:1", Relation: "accepted-proposal"}}}
	if got, err := acceptancesInDiff(bg, app, base, aliased); err != nil || len(got) != 1 || got[0] != "path:e2e/a.spec.ts:1" {
		t.Fatalf("aliased acceptances in diff %v %v", got, err)
	}
}

// CCE-V0-131: harvest reads the test root at its pin and records it; a two-repo harvest whose map
// does not build refuses instead of guessing the test root.
func TestCCEV0131TwoRepoHarvest(t *testing.T) {
	app, base, e2e, e2eRev := twoRepoFixture(t)
	commitFiles(t, e2e, map[string]*string{"e2e/specs/literal.spec.ts": text("await page.goto('/clubs/1/teesheets');\n")}, "literal")
	e2eRev = gitTest(t, e2e, "rev-parse", "HEAD")
	roots := appmap.Roots{Repos: map[string]string{"e2e": e2e}}
	p, err := Harvest(bg, HarvestOptions{Root: app, Rev: base, Manifest: "appmap.json", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if p.Revision != base || len(p.Roots) != 1 || p.Roots[0].Revision != e2eRev || p.Files == 0 || len(p.Proposals) == 0 {
		t.Fatalf("%s", p.Encoded())
	}
	for _, pr := range p.Proposals {
		if pr.Block.Span.Repo != "e2e" {
			t.Fatalf("proposal without the test root alias: %+v", pr)
		}
	}
	bad := appmap.Roots{Repos: map[string]string{"e2e": t.TempDir()}}
	if _, err := Harvest(bg, HarvestOptions{Root: app, Rev: base, Manifest: "appmap.json", Roots: bad}); refusalCode(err) != "e2e-map-unavailable" {
		t.Fatalf("unbuildable two-repo map: %v", err)
	}
}

// CCE-V0-135: compile takes the diff in --root and emits scenarios from the blocks read in the
// aliased tests root at its pin, recording that pin and exposing the tests root to prove.
func TestCCEV0135TwoTreeCompile(t *testing.T) {
	app, base, e2e, e2eRev := twoRepoFixture(t)
	head := commitBehaviourChange(t, app)
	roots := appmap.Roots{Repos: map[string]string{"e2e": e2e}, Revisions: map[string]string{"e2e": e2eRev}}
	r, err := Compile(bg, DiffOptions{Root: app, Base: base, Head: head, Manifest: "appmap.json", Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if r.State != Compiled || len(r.Compiled()) != 1 || len(r.Roots) != 1 || r.Roots[0].Repo != "e2e" || r.Roots[0].Revision != e2eRev {
		t.Fatalf("%s", r.Encoded())
	}
	if tr := r.TestsRoot(); tr == nil || tr.Repo != "e2e" || tr.Dir != e2e || tr.Revision != e2eRev {
		t.Fatalf("tests root %+v", tr)
	}
	root, oneBase := fixtureRepo(t)
	if one, err := Compile(bg, diffOpts(root, oneBase, commitBehaviourChange(t, root))); err != nil || one.TestsRoot() != nil || len(one.Roots) != 0 {
		t.Fatalf("a one-tree compile names no tests root: %v", err)
	}
}

// CCE-V0-136: a two-tree proof reads the test tree from the tests root at its pin and runs it over
// --root's head and base; both sides record the tests revision. A test-tree file that would
// replace different application bytes reads INCONCLUSIVE before any run, and a hosted test with an
// aliased tests root refuses.
func TestCCEV0136TwoTreeProve(t *testing.T) {
	app, base, e2e, _ := twoRepoFixture(t)
	page := strings.Replace(*overlayText(t, "e2e/pages/club.page.ts"), "async cleanup() {", "// tests-root page object\n  async cleanup() {", 1)
	e2eRev := commitFiles(t, e2e, map[string]*string{"e2e/pages/club.page.ts": text(page)}, "suite page object")
	head := commitBehaviourChange(t, app)
	roots := appmap.Roots{Repos: map[string]string{"e2e": e2e}, Revisions: map[string]string{"e2e": e2eRev}}
	r, err := Compile(bg, DiffOptions{Root: app, Base: base, Head: head, Manifest: "appmap.json", Roots: roots})
	if err != nil || len(r.Compiled()) != 1 {
		t.Fatalf("compile: %v", err)
	}
	sc := r.Compiled()[0]
	tests := testsRoot{dir: e2e, rev: e2eRev, repo: "e2e"}
	f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("passed")}}, controls: "killed"}
	e := f.env(app)
	e.tests = tests
	p := e.proveScenario(bg, base, head, sc, 3)
	if f.called != 2 {
		t.Fatalf("runner called %d times: %+v", f.called, p)
	}
	for _, rev := range []string{head, base} {
		if f.staged[rev]["e2e/pages/club.page.ts"] != page {
			t.Fatalf("side %s did not run the tests root's page object", rev)
		}
	}
	if p.Head.TestRevision != e2eRev || p.Base.TestRevision != e2eRev || p.Head.ApplicationRevision != head || p.Base.ApplicationRevision != base {
		t.Fatalf("sides %+v / %+v", p.Head, p.Base)
	}
	if strings.Contains(f.staged[base]["app/components/teesheet.html"], "Booked") {
		t.Fatal("the base side must run the base application tree")
	}

	shadow := commitFiles(t, app, map[string]*string{"e2e/pages/club.page.ts": text("export class ClubPage {}\n")}, "application shadows the page object")
	g := &fakeRunner{controls: "killed"}
	e = g.env(app)
	e.tests = tests
	if p := e.proveScenario(bg, base, shadow, sc, 3); p.Verdict != VerdictInconclusive || p.Note != ReasonTestTreeOverlaps ||
		p.Head.TestRevision != e2eRev || g.called != 0 {
		t.Fatalf("shadowed test tree: %s %s, calls %d", p.Verdict, p.Note, g.called)
	}
	// An application file the scenario's `../pages/club.page` import resolves to ahead of the tests
	// root's club.page.ts changes the code that runs without sharing a path with the tree.
	resolved := commitFiles(t, app, map[string]*string{"e2e/pages/club.page.ts": nil, "e2e/pages/club.page": text("export class ClubPage {}\n")},
		"application shadows the page object's resolution")
	g = &fakeRunner{controls: "killed"}
	e = g.env(app)
	e.tests = tests
	if p := e.proveScenario(bg, base, resolved, sc, 3); p.Verdict != VerdictInconclusive || p.Note != ReasonTestTreeOverlaps || g.called != 0 {
		t.Fatalf("resolution-shadowed test tree: %s %s, calls %d", p.Verdict, p.Note, g.called)
	}

	_, err = Prove(bg, ProveOptions{Root: app, Base: base, Head: head, Hosted: &HostedSelection{File: "e2e/specs/a.spec.ts"},
		Tests: &TestsRoot{Repo: "e2e", Dir: e2e, Revision: e2eRev}})
	if refusalCode(err) != "e2e-roots-unsupported" {
		t.Fatalf("hosted with an aliased tests root: %v", err)
	}
}
