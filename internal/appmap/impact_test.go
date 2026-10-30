package appmap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func impactRun(t *testing.T, f prefixFixture, l *EdgeLedger, base string, o Options) ImpactReport {
	t.Helper()
	o.Root = f.root
	raw, err := Impact(context.Background(), f.m, l, base, o)
	if err != nil {
		t.Fatal(err)
	}
	var r ImpactReport
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func impactCommit(t *testing.T, f prefixFixture, files map[string]string) {
	t.Helper()
	for p, body := range files {
		dst := filepath.Join(f.root, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitAll(t, f.root, "change")
}

// ANL-V0-015: a router-file change invalidates its screen and every descendant, lists the touched
// edges with the ledger's flipped status and cheapest re-proving test, and names spec files.
func TestANLV0015RouterChangeReachesDescendants(t *testing.T) {
	f := prefixSetup(t)
	for i := range f.m.Screens {
		switch s := &f.m.Screens[i]; s.ID {
		case f.a:
			s.Anchor.Path, s.Parent = "router/parent.ts", ""
		case f.b:
			s.Anchor.Path, s.Parent = "router/child.ts", f.a
		case f.c:
			s.Anchor.Path, s.Parent = "router/other.ts", f.b
		}
	}
	base, err := (repo{ctx: context.Background(), root: f.root}).resolve("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	impactCommit(t, f, map[string]string{"router/parent.ts": "export {}\n"})
	l := provenLedger(f, [2]string{f.a, f.b})
	l.Revision = gitTest(t, f.root, "rev-parse", "HEAD")
	l.Edges[0].Status = "UNVERIFIED_AT_HEAD"
	r := impactRun(t, f, l, base, Options{})
	if r.Verdict != VerdictTargeted || r.Ignored != 0 || len(r.Unknown) != 0 {
		t.Fatalf("%+v", r)
	}
	got := map[string]string{}
	for _, s := range r.Screens {
		got[s.ID] = s.Basis
	}
	if got[f.a] != "router-anchor" || got[f.b] != "lineage:"+f.a || got[f.c] != "lineage:"+f.a {
		t.Fatalf("%v a=%s b=%s c=%s", got, f.a, f.b, f.c)
	}
	if len(r.Edges) != 1 || !r.Edges[0].Flipped || r.Edges[0].Reprove != "login-test" || len(r.TestKeys) != 1 {
		t.Fatalf("%+v", r.Edges)
	}
	if len(r.SpecFiles) == 0 {
		t.Fatal("no spec files for touched sequence edges")
	}
	for _, p := range r.SpecFiles {
		if strings.HasPrefix(p, "file:") {
			t.Fatal("element ID reported as a path", r.SpecFiles)
		}
	}
	l.MapDigest = "sha256:other"
	if _, err := Impact(context.Background(), f.m, l, base, Options{Root: f.root}); err == nil {
		t.Fatal("foreign ledger accepted")
	}
}

// ANL-V0-016: an unplaceable UI path forces full-suite-required, a bad base fails closed, and an
// empty change carries no navigation impact (a docs-only change is unknown, ANL-V0-048).
func TestANLV0016UnknownForcesFullSuite(t *testing.T) {
	f := prefixSetup(t)
	base, _ := (repo{ctx: context.Background(), root: f.root}).resolve("HEAD")
	if r := impactRun(t, f, nil, base, Options{}); r.Verdict != VerdictNoNavigation || r.Changed != 0 || r.Ignored != 0 {
		t.Fatalf("%+v", r)
	}
	impactCommit(t, f, map[string]string{"app/templates/club.html": "<div></div>\n"})
	r := impactRun(t, f, nil, base, Options{})
	if r.Verdict != VerdictFullSuite || len(r.Unknown) != 1 || r.Unknown[0].Reason != "unmapped-path" {
		t.Fatalf("%+v", r)
	}
	f.m.Screens[0].Specs = append(f.m.Screens[0].Specs, Attribution{File: "app/shared.ts", Basis: "import"})
	f.m.Files = append(f.m.Files, TestFile{ID: "file:alias", Path: "app/shared.ts", Role: roleSpec, Anchor: Anchor{Repo: "tests"}})
	impactCommit(t, f, map[string]string{"app/shared.ts": "export {}\n"})
	if r = impactRun(t, f, nil, base, Options{}); len(r.Unknown) != 2 {
		t.Fatalf("aliased-repo test file placed a primary-repo path: %+v", r)
	}
	if _, err := Impact(context.Background(), f.m, nil, "-bad", Options{Root: f.root}); err == nil {
		t.Fatal("bad base accepted")
	}
	small := impactRun(t, f, nil, base, Options{Budget: 1000})
	if small.Verdict != VerdictFullSuite || small.Digest != r.Digest || small.Omitted["unknown"]+small.Omitted["screens"]+small.Omitted["spec_files"]+small.Omitted["test_keys"] == 0 {
		t.Fatalf("%+v", small)
	}
}

// impactReasons maps each unknown path of a report to its reason.
func impactReasons(r ImpactReport) map[string]string {
	got := map[string]string{}
	for _, u := range r.Unknown {
		got[u.Path] = u.Reason
	}
	return got
}

// ANL-V0-048: a Ruby-only change (an edited model and a new worker) and a docs-only change are
// placed by nothing, so each path reads unknown `non-ui-path` and the verdict is
// full-suite-required; nothing is counted under ignored_paths (V1-2697, issue #1803).
func TestANLV0048RubyOnlyChangeFailsClosed(t *testing.T) {
	f := prefixSetup(t)
	base, _ := (repo{ctx: context.Background(), root: f.root}).resolve("HEAD")
	impactCommit(t, f, map[string]string{"app/models/club.rb": "class Club; end\n", "app/workers/sync_worker.rb": "class SyncWorker; end\n"})
	r := impactRun(t, f, nil, base, Options{})
	got := impactReasons(r)
	if r.Verdict != VerdictFullSuite || r.Changed != 2 || r.Ignored != 0 || len(got) != 2 ||
		got["app/models/club.rb"] != "non-ui-path" || got["app/workers/sync_worker.rb"] != "non-ui-path" || len(r.SpecFiles) != 0 {
		t.Fatalf("Ruby-only change narrowed: %+v", r)
	}
	head := gitTest(t, f.root, "rev-parse", "HEAD")
	impactCommit(t, f, map[string]string{"app/models/club.rb": "class Club\n  def x; end\nend\n"})
	if r = impactRun(t, f, nil, head, Options{}); r.Verdict != VerdictFullSuite || impactReasons(r)["app/models/club.rb"] != "non-ui-path" {
		t.Fatalf("one-line model change narrowed: %+v", r)
	}
	head = gitTest(t, f.root, "rev-parse", "HEAD")
	impactCommit(t, f, map[string]string{"README.md": "docs\n"})
	if r = impactRun(t, f, nil, head, Options{}); r.Verdict != VerdictFullSuite || r.Ignored != 0 || impactReasons(r)["README.md"] != "non-ui-path" {
		t.Fatalf("docs-only change ignored without a declared exemption: %+v", r)
	}
}

// ANL-V0-049: a page-object-only change that no manifest binding places on a screen selects the
// specs importing it, directly and through a workflow; while the test import graph has a file or
// import the map could not resolve, the path reads unknown `test-import-graph-incomplete` and the
// verdict is full-suite-required; a support file nothing imports reads `test-support-unreached`
// (V1-2697, issue #1803).
func TestANLV0049PageObjectOnlyChangeSelectsImportingSpecs(t *testing.T) {
	root, _ := fixtureRepo(t)
	files := map[string]string{
		"e2e/pages/table.page.ts":      "import { Page } from '@playwright/test';\nexport class TablePage {\n  constructor(private readonly page: Page) {}\n  async sort() {\n    await this.page.getByTestId('sort').click();\n  }\n}\n",
		"e2e/workflows/sorting.ts":     "import { Page } from '@playwright/test';\nimport { TablePage } from '../pages/table.page';\nexport async function sortTable(page: Page) {\n  await new TablePage(page).sort();\n}\n",
		"e2e/specs/table.spec.ts":      "import { test } from '@playwright/test';\nimport { TablePage } from '../pages/table.page';\ntest('sorts', async ({ page }) => {\n  await new TablePage(page).sort();\n});\n",
		"e2e/specs/table-flow.spec.ts": "import { test } from '@playwright/test';\nimport { sortTable } from '../workflows/sorting';\ntest('sorts through the workflow', async ({ page }) => {\n  await sortTable(page);\n});\n",
		"e2e/specs/unrelated.spec.ts":  "import { test } from '@playwright/test';\ntest('stands alone', async ({ page }) => {\n  await page.goto('/#!/login');\n});\n",
	}
	for p, body := range files {
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rev := commitAll(t, root, "unbound page object")
	f := prefixFixture{m: build(t, root, rev), root: root, rev: rev}
	if po := f.m.file("e2e/pages/table.page.ts"); po == nil || po.Role != rolePageObject || po.Screen != "" {
		t.Fatalf("fixture page object is bound: %+v", po)
	}
	if !testGraphIncomplete(f.m) {
		t.Fatal("fixture graph should hold the unresolved and alias specs")
	}
	want := []string{"e2e/specs/table-flow.spec.ts", "e2e/specs/table.spec.ts"}
	impactCommit(t, f, map[string]string{"e2e/pages/table.page.ts": "export class TablePage {}\n"})
	r := impactRun(t, f, nil, rev, Options{})
	if r.Verdict != VerdictFullSuite || !reflect.DeepEqual(r.SpecFiles, want) || len(r.Unknown) != 1 ||
		r.Unknown[0].Path != "e2e/pages/table.page.ts" || r.Unknown[0].Reason != "test-import-graph-incomplete" {
		t.Fatalf("page object behind an incomplete import graph narrowed: %+v", r)
	}

	gitTest(t, root, "rm", "-q", "e2e/specs/unresolved.spec.ts", "e2e/specs/alias.spec.ts")
	clean := commitAll(t, root, "drop unresolved specs")
	f = prefixFixture{m: build(t, root, clean), root: root, rev: clean}
	if testGraphIncomplete(f.m) {
		t.Fatalf("fixture graph still incomplete: %+v", f.m.Unknowns)
	}
	impactCommit(t, f, map[string]string{"e2e/pages/table.page.ts": "export class TablePage { v = 2 }\n"})
	r = impactRun(t, f, nil, clean, Options{})
	if r.Verdict == VerdictFullSuite || !reflect.DeepEqual(r.SpecFiles, want) || len(r.Unknown) != 0 {
		t.Fatalf("page-object-only change did not select its importing specs: %+v", r)
	}
	if err := os.WriteFile(filepath.Join(root, "e2e/pages/orphan.page.ts"), []byte("export class Orphan {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orphan := commitAll(t, root, "orphan page object")
	f = prefixFixture{m: build(t, root, orphan), root: root, rev: orphan}
	impactCommit(t, f, map[string]string{"e2e/pages/orphan.page.ts": "export class Orphan { v = 2 }\n"})
	r = impactRun(t, f, nil, orphan, Options{})
	if r.Verdict != VerdictFullSuite || impactReasons(r)["e2e/pages/orphan.page.ts"] != "test-support-unreached" {
		t.Fatalf("unimported page object narrowed: %+v", r)
	}
}

// ANL-V0-049: on an otherwise complete test import graph, a computed `import(name)` or a
// resolved `.mts` module the map records no test file for makes the graph incomplete; a changed
// UI file whose reference hop ends at a bound page object also selects the specs importing it.
func TestANLV0049GraphIncompleteSourcesAndHopImporters(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/unresolved.spec.ts", "e2e/specs/alias.spec.ts")
	home, err := os.ReadFile(filepath.Join(root, "e2e/pages/home.page.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for p, body := range map[string]string{
		"e2e/pages/home.page.ts": string(home) + "export const banner = '../../app/home/banner.html';\n",
		"app/home/banner.html":   "<p></p>\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	clean := commitAll(t, root, "complete graph")
	f := prefixFixture{m: build(t, root, clean), root: root, rev: clean}
	if testGraphIncomplete(f.m) {
		t.Fatalf("fixture graph incomplete: %+v", f.m.Unknowns)
	}
	if po := f.m.file("e2e/pages/home.page.ts"); po == nil || po.Screen == "" {
		t.Fatalf("home page object is not bound: %+v", po)
	}
	impactCommit(t, f, map[string]string{"app/home/banner.html": "<p>v2</p>\n"})
	r := impactRun(t, f, nil, clean, Options{})
	if !slices.Contains(r.SpecFiles, "e2e/specs/teesheet-click.spec.ts") || len(r.Unknown) != 0 {
		t.Fatalf("hop to a bound page object did not select its importing specs: %+v", r)
	}
	for name, files := range map[string]map[string]string{
		"computed": {"e2e/specs/computed.spec.ts": "import { test } from '@playwright/test';\nconst name = './dyn';\ntest('loads', async () => {\n  await import(name);\n});\n"},
		"mts": {
			"e2e/lib/helpers.mts":   "export const helper = 1;\n",
			"e2e/specs/esm.spec.ts": "import { test } from '@playwright/test';\nimport { helper } from '../lib/helpers.mts';\ntest('esm', async () => { void helper; });\n",
		},
	} {
		gitTest(t, root, "checkout", "-q", clean)
		for p, body := range files {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		rev := commitAll(t, root, name)
		f = prefixFixture{m: build(t, root, rev), root: root, rev: rev}
		if !testGraphIncomplete(f.m) {
			t.Fatalf("%s: graph read complete: %+v", name, f.m.Unknowns)
		}
		impactCommit(t, f, map[string]string{"app/home/banner.html": "<p>v3</p>\n"})
		r = impactRun(t, f, nil, rev, Options{})
		if r.Verdict != VerdictFullSuite || impactReasons(r)["app/home/banner.html"] != "test-import-graph-incomplete" {
			t.Fatalf("%s: hop through a page object narrowed on an incomplete graph: %+v", name, r)
		}
	}
}

// ANL-V0-032: a changed template or component the map does not anchor is walked through the
// string literals that name it up to the router span that references it; a mention the walk
// cannot resolve, or a file nothing references, keeps the path unknown.
func TestANLV0032TemplateHopPlacesReferencedFiles(t *testing.T) {
	root, _ := fixtureRepo(t)
	routes, err := os.ReadFile(filepath.Join(root, "app/routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := os.ReadFile(filepath.Join(root, "e2e/scenarios/club-with-slots.ts"))
	if err != nil {
		t.Fatal(err)
	}
	src := "var names = {};\nvar dyn = require('./' + names.dyn);\nvar shell = require('./shell/shell.html');\nvar widgets = require('./widgets');\nvar label = \"prefix\\\"./esc/esc.html\";\n// was require('./ghost/ghost.html')\n/* or\n  './ghost/ghost2.html' */\n" + string(routes)
	src = strings.Replace(src, "url: '/settings'", "url: '/settings',\n      templateUrl: 'app/clubs/settings.html'", 1)
	src = strings.Replace(src, "url: '/members'\n    })\n    .state('members2'", "url: '/members',\n      controller: require('./members/list.component')\n    })\n    .state('members2'", 1)
	files := map[string]string{
		"app/routes.js":                    src,
		"app/shell/shell.html":             "<nav></nav>\n",
		"app/clubs/settings.html":          "<form></form>\n",
		"app/members/list.component.js":    "module.exports = { templateUrl: './row.html' };\n",
		"app/members/row.html":             "<tr></tr>\n",
		"app/shared/badge.html":            "<b></b>\n",
		"app/shared/badge.js":              "// renders badge.html\nmodule.exports = { templateUrl: './badge.html' };\n",
		"app/lonely.js":                    "module.exports = './orphan.html';\n",
		"app/orphan.html":                  "<i></i>\n",
		"app/ghost/ghost.html":             "<p></p>\n",
		"app/ghost/ghost2.html":            "<p></p>\n",
		"app/cyc/alpha.js":                 "module.exports = require('./beta');\n",
		"app/cyc/beta.js":                  "module.exports = require('./alpha');\n",
		"app/esc/esc.html":                 "<p></p>\n",
		"app/tpl/tpl.html":                 "<p></p>\n",
		"app/tplhost2.js":                  "var rx = /\"/; var tpl = `\" './tpl2/tpl2.html'`;\n",
		"app/tpl2/tpl2.html":               "<p></p>\n",
		"app/view.hbs":                     "{{!--\n{{foo}}\n'./hbs/hbs.html'\n--}}\n",
		"app/hbs/hbs.html":                 "<p></p>\n",
		"app/tplhost.js":                   "var rx = /\"/; var tpl = `\n'./tpl/tpl.html'\n`;\n",
		"app/dyn.js":                       "module.exports = 1;\n",
		"app/dynuser.js":                   "module.exports = require('./dyn');\n",
		"app/helped/helped.html":           "<p></p>\n",
		"e2e/scenarios/club-with-slots.ts": string(scenario) + "export const helped = '../../app/helped/helped.html';\n",
		"app/widgets/index.js":             "module.exports = {};\n",
		"app/widgets/local.js":             "module.exports = require('./index');\n",
	}
	for p, body := range files {
		dst := filepath.Join(root, p)
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rev := commitAll(t, root, "templates")
	f := prefixFixture{m: build(t, root, rev), root: root, rev: rev}
	settings, members := screenID("admin", "app.clubs.settings"), screenID("admin", "app.clubs.members")
	bases := func(r ImpactReport) map[string]string {
		got := map[string]string{}
		for _, s := range r.Screens {
			got[s.ID] = s.Basis
		}
		return got
	}
	impactCommit(t, f, map[string]string{"app/clubs/settings.html": "<form>v2</form>\n"})
	r := impactRun(t, f, nil, rev, Options{})
	if got := bases(r); r.Verdict != VerdictTargeted || len(got) != 1 || got[settings] != "template-reference:app/routes.js" {
		t.Fatalf("template in a state span: %+v", r)
	}
	impactCommit(t, f, map[string]string{"app/members/row.html": "<tr>v2</tr>\n"})
	r = impactRun(t, f, nil, rev, Options{})
	if got := bases(r); r.Verdict != VerdictTargeted || len(got) != 2 || got[members] != "template-reference:app/routes.js" {
		t.Fatalf("template through a component: %+v", r)
	}
	impactCommit(t, f, map[string]string{"app/shared/badge.html": "<b>v2</b>\n"})
	r = impactRun(t, f, nil, rev, Options{})
	if r.Verdict != VerdictFullSuite || len(r.Unknown) != 1 || r.Unknown[0].Path != "app/shared/badge.html" || r.Unknown[0].Reason != "unresolved-reference" {
		t.Fatalf("unresolved mention narrowed: %+v", r)
	}
	head := gitTest(t, root, "rev-parse", "HEAD")
	impactCommit(t, f, map[string]string{"app/orphan.html": "<i>v2</i>\n"})
	r = impactRun(t, f, nil, head, Options{})
	if r.Verdict != VerdictFullSuite || len(r.Unknown) != 1 || r.Unknown[0].Reason != "unmapped-path" {
		t.Fatalf("unbound root narrowed: %+v", r)
	}
	// Commented, escaped and markup mentions, an import cycle, and an index file one referrer reaches by
	// directory while another, which nothing references, names ./index (the `widgets` identifier is no
	// mention since ANL-V0-070, so the walk ends at that unreferenced file).
	for p, c := range map[string][2]string{
		"app/ghost/ghost.html":  {"unresolved-reference", "<p>v2</p>\n"},
		"app/ghost/ghost2.html": {"unresolved-reference", "<p>v2</p>\n"},
		"app/esc/esc.html":      {"unresolved-reference", "<p>v2</p>\n"},
		"app/tpl/tpl.html":      {"unresolved-reference", "<p>v2</p>\n"},
		"app/hbs/hbs.html":      {"unresolved-reference", "<p>v2</p>\n"},
		"app/tpl2/tpl2.html":    {"unresolved-reference", "<p>v2</p>\n"},
		"app/dyn.js":            {"unresolved-reference", "module.exports = 2;\n"},
		"app/cyc/alpha.js":      {"unmapped-path", "module.exports = require('./beta'); // v2\n"},
		"app/widgets/index.js":  {"unmapped-path", "module.exports = { v: 2 };\n"},
	} {
		head = gitTest(t, root, "rev-parse", "HEAD")
		impactCommit(t, f, map[string]string{p: c[1]})
		r = impactRun(t, f, nil, head, Options{})
		if r.Verdict != VerdictFullSuite || len(r.Unknown) != 1 || r.Unknown[0].Path != p || r.Unknown[0].Reason != c[0] {
			t.Fatalf("%s narrowed: %+v", p, r)
		}
	}
	// A map test file selecting nothing (a scenario helper) is walked through, never a terminal.
	head = gitTest(t, root, "rev-parse", "HEAD")
	impactCommit(t, f, map[string]string{"app/helped/helped.html": "<p>v2</p>\n"})
	r = impactRun(t, f, nil, head, Options{})
	if len(r.SpecFiles)+len(r.Unknown) == 0 {
		t.Fatalf("a helper-only reference narrowed: %+v", r)
	}
	head = gitTest(t, root, "rev-parse", "HEAD")
	impactCommit(t, f, map[string]string{"app/shell/shell.html": "<nav>v2</nav>\n"})
	r = impactRun(t, f, nil, head, Options{})
	anchored := 0
	for _, s := range f.m.Screens {
		if s.Anchor.Path == "app/routes.js" && s.Anchor.Repo == "" {
			anchored++
		}
	}
	if r.Verdict != VerdictTargeted || len(r.Screens) < anchored || anchored < 5 {
		t.Fatalf("reference outside every span did not widen to the file (%d anchored): %+v", anchored, r)
	}
}

// ANL-V0-047: an index file's `index` token in code position (a loop index, a property, an object
// key) in a confidently lexed script is not a reference, so a change to an index file that the
// router reaches by its directory resolves to that reference instead of unresolved-reference
// (V1-1216). A commented `index` or one in markup still refuses.
func TestANLV0047IndexIdentifiersDoNotHideTheIndexFileReference(t *testing.T) {
	root, _ := fixtureRepo(t)
	routes, err := os.ReadFile(filepath.Join(root, "app/routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"app/routes.js":       "var pane = require('./panels');\n" + string(routes),
		"app/panels/index.js": "module.exports = {};\n",
		"app/list.js":         "var rows = [{ index: 0 }];\nrows.forEach(function (row, index) { return row.index + index; });\n",
	}
	for p, body := range files {
		dst := filepath.Join(root, p)
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rev := commitAll(t, root, "index identifiers")
	f := prefixFixture{m: build(t, root, rev), root: root, rev: rev}
	impactCommit(t, f, map[string]string{"app/panels/index.js": "module.exports = { v: 2 };\n"})
	r := impactRun(t, f, nil, rev, Options{})
	if r.Verdict != VerdictTargeted || len(r.Unknown) != 0 || len(r.Screens) == 0 || r.Screens[0].Basis != "template-reference:app/routes.js" {
		t.Fatalf("index file behind index identifiers not placed: %+v", r)
	}
	for _, c := range []struct{ name, body, clean string }{
		{"app/note.js", "// see index\nmodule.exports = 1;\n", "module.exports = 1;\n"},
		{"app/list.html", "<li>{{index}}</li>\n", "<li></li>\n"},
	} {
		name := c.name
		head := gitTest(t, root, "rev-parse", "HEAD")
		impactCommit(t, f, map[string]string{name: c.body, "app/panels/index.js": "module.exports = { v: '" + name + "' };\n"})
		r = impactRun(t, f, nil, head, Options{})
		unresolved := false
		for _, u := range r.Unknown {
			unresolved = unresolved || u.Path == "app/panels/index.js" && u.Reason == "unresolved-reference"
		}
		if r.Verdict != VerdictFullSuite || !unresolved {
			t.Fatalf("%s: an unprovable index mention narrowed: %+v", name, r)
		}
		impactCommit(t, f, map[string]string{name: c.clean})
	}
}
