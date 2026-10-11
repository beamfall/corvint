package appmap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	impactCommit(t, f, map[string]string{"router/parent.ts": "export {}\n", "notes.md": "x\n"})
	l := provenLedger(f, [2]string{f.a, f.b})
	l.Revision = gitTest(t, f.root, "rev-parse", "HEAD")
	l.Edges[0].Status = "UNVERIFIED_AT_HEAD"
	r := impactRun(t, f, l, base, Options{})
	if r.Verdict != VerdictTargeted || r.Ignored != 1 || len(r.Unknown) != 0 {
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

// ANL-V0-016: an unplaceable UI path forces full-suite-required, docs-only changes carry no
// navigation impact, and a bad base fails closed.
func TestANLV0016UnknownForcesFullSuite(t *testing.T) {
	f := prefixSetup(t)
	base, _ := (repo{ctx: context.Background(), root: f.root}).resolve("HEAD")
	impactCommit(t, f, map[string]string{"README.md": "docs\n"})
	if r := impactRun(t, f, nil, base, Options{}); r.Verdict != VerdictNoNavigation || r.Changed != 1 || r.Ignored != 1 {
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
	small := impactRun(t, f, nil, base, Options{Budget: 700})
	if small.Verdict != VerdictFullSuite || small.Digest != r.Digest {
		t.Fatalf("%+v", small)
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
	src := "var tpl = `\n'./tpl/tpl.html'\n`;\nvar shell = require('./shell/shell.html');\nvar widgets = require('./widgets');\nvar label = \"prefix\\\"./esc/esc.html\";\n// was require('./ghost/ghost.html')\n/* or\n  './ghost/ghost2.html' */\n" + string(routes)
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
	// Commented and escaped mentions, an import cycle, and an index file one referrer reaches by
	// directory while another names ./index (any bare `index` word elsewhere is unresolved).
	for p, c := range map[string][2]string{
		"app/ghost/ghost.html":  {"unresolved-reference", "<p>v2</p>\n"},
		"app/ghost/ghost2.html": {"unresolved-reference", "<p>v2</p>\n"},
		"app/esc/esc.html":      {"unresolved-reference", "<p>v2</p>\n"},
		"app/tpl/tpl.html":      {"unresolved-reference", "<p>v2</p>\n"},
		"app/cyc/alpha.js":      {"unmapped-path", "module.exports = require('./beta'); // v2\n"},
		"app/widgets/index.js":  {"unresolved-reference", "module.exports = { v: 2 };\n"},
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
