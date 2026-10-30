package changee2e

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// requireRemedies fails unless every unknown span carries a remedy that names a key or states its
// uncertainty: a remedy is never empty and never silently missing (CCE-V0-150).
func requireRemedies(t *testing.T, o *Obligations) {
	t.Helper()
	for _, u := range o.Unknown {
		if u.Remedy == nil || (u.Remedy.Key == "" && u.Remedy.Uncertainty == "") || (u.Remedy.Key == "" && len(u.Remedy.Candidates) != 0) {
			t.Fatalf("%s %s: remedy %+v", u.Reason, u.Span.Path, u.Remedy)
		}
	}
}

// CCE-V0-150: each unknown span names the declaration key and inferred candidates that would bound
// it (reach_exempt for a non-UI path, page_object_screens for an ambiguous page object, a flow
// outcome for a reached screen without one), or states its uncertainty with no key.
func TestCCEV0150UnknownSpanRemedy(t *testing.T) {
	root, base := fixtureRepo(t)
	ambig := "import { Page } from '@playwright/test';\n\nexport class AmbigPage {\n  constructor(private readonly page: Page) {}\n\n" +
		"  async home() {\n    await this.page.goto('/#!/home');\n  }\n\n  async settings() {\n    await this.page.goto('/#!/clubs/1/settings');\n  }\n}\n"
	head := commitFiles(t, root, map[string]*string{"server/handler.go": text("package server\n"), "e2e/pages/ambig.page.ts": &ambig}, "server and page")
	o := buildObligations(t, root, base, head)
	requireRemedies(t, o)
	u := unknownWith(o, ReasonUnplacedPath, "server/handler.go")
	if u == nil || u.Detail != "non-ui-path" || u.Remedy.File != DeclarationsPath || u.Remedy.Key != RemedyKeyReachExempt ||
		!slices.Equal(u.Remedy.Candidates, []string{"server/handler.go"}) || u.Remedy.Uncertainty == "" {
		t.Fatalf("non-UI remedy: %s", o.Encoded())
	}
	u = unknownWith(o, ReasonMapUnknown, "e2e/pages/ambig.page.ts")
	if u == nil || u.Remedy.Key != appmap.RemedyKeyPageObjectScreens || !slices.Equal(u.Remedy.Candidates, []string{
		`{"path":"e2e/pages/ambig.page.ts","state":"app.clubs.settings"}`, `{"path":"e2e/pages/ambig.page.ts","state":"app.home"}`}) ||
		!strings.Contains(u.Remedy.Uncertainty, "declare the one it models") {
		t.Fatalf("ambiguous page object remedy: %s", o.Encoded())
	}
	if globEscape("a/[x]*?.go") != `a/\[x]\*\?.go` {
		t.Fatalf("glob escape %q", globEscape("a/[x]*?.go"))
	}

	// A reached screen without an outcome: the flows key with the screen as the candidate.
	routes := strings.Replace(*overlayText(t, "app/routes.js"), "url: '/settings'",
		"url: '/settings',\n      templateUrl: 'app/components/settings.html'", 1)
	sBase := commitFiles(t, root, map[string]*string{"app/routes.js": &routes, "app/components/settings.html": text("<h1>Settings</h1>\n")}, "settings")
	sHead := commitFiles(t, root, map[string]*string{"app/components/settings.html": text("<h1>Club settings</h1>\n")}, "label")
	o = buildObligations(t, root, sBase, sHead)
	requireRemedies(t, o)
	u = unknownWith(o, ReasonScreenWithoutOutcome, "app/components/settings.html")
	if u == nil || u.Remedy.Key != RemedyKeyFlows || !slices.Equal(u.Remedy.Candidates, []string{"screen:admin:app.clubs.settings"}) {
		t.Fatalf("outcome remedy: %s", o.Encoded())
	}

	// Map unknowns no declaration resolves state their uncertainty and name no key.
	more := *overlayText(t, "app/routes.js") + "// reviewed\n"
	o = buildObligations(t, root, base, commitFiles(t, root, map[string]*string{"app/routes.js": &more}, "routes"))
	requireRemedies(t, o)
	for _, u := range o.Unknown {
		if u.Reason == ReasonMapUnknown && strings.Contains(u.Detail, "non-literal-url") && (u.Remedy.Key != "" || !strings.Contains(u.Remedy.Uncertainty, "no declaration change")) {
			t.Fatalf("invented remedy %+v", u.Remedy)
		}
	}
}

// CCE-V0-151: the range summary evaluates each first-parent commit against its parent and ranks
// the keyed remedies by the merges they alone bound (a remedy beside a keyless unknown bounds
// none); a merge with a keyless unknown is
// unremediable, a refusal is recorded, and --coverage or --ledger refuse.
func TestCCEV0151RemedySummary(t *testing.T) {
	root, base := fixtureRepo(t)
	c1 := commitFiles(t, root, map[string]*string{"server/a.go": text("package server\n")}, "a")
	c2 := commitFiles(t, root, map[string]*string{"server/a.go": text("package server // 2\n")}, "a2")
	c3 := commitFiles(t, root, map[string]*string{"server/b.go": text("package server\n")}, "b")
	more := *overlayText(t, "app/routes.js") + "// reviewed\n"
	c4 := commitFiles(t, root, map[string]*string{"app/routes.js": &more}, "routes")
	raw, err := BuildRemedySummary(bg, diffOpts(root, base, c4), MaxRemedyCommits)
	if err != nil {
		t.Fatal(err)
	}
	var s RemedySummary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s.Schema != RemedySummarySchema || s.Evaluated != 4 || s.OmittedCommits != 0 || s.Unremediable != 1 || s.Digest == "" || len(s.Remedies) != 3 {
		t.Fatalf("%s", raw)
	}
	a, b := s.Remedies[0], s.Remedies[1]
	if a.Remedy.Key != RemedyKeyReachExempt || a.Remedy.Candidates[0] != "server/a.go" || a.Bounds != 2 || !slices.Equal(a.Commits, []string{c1, c2}) ||
		b.Remedy.Candidates[0] != "server/b.go" || b.Bounds != 1 || !slices.Equal(b.Commits, []string{c3}) ||
		s.Remedies[2].Bounds != 0 || !slices.Equal(s.Remedies[2].Commits, []string{c4}) {
		t.Fatalf("ranking: %s", raw)
	}
	if m := s.Merges[3]; m.Commit != c4 || !m.Unremediable || m.BoundedBy != "" || s.Merges[0].BoundedBy != a.ID {
		t.Fatalf("merges: %s", raw)
	}
	raw, err = BuildRemedySummary(bg, diffOpts(root, base, c4), 2)
	var bounded RemedySummary
	if err != nil || json.Unmarshal(raw, &bounded) != nil || bounded.OmittedCommits != 2 || bounded.Evaluated != 2 || bounded.Merges[0].Commit != c3 {
		t.Fatalf("bounded summary: %v %s", err, raw)
	}
	var r *Refusal
	o := diffOpts(root, base, c4)
	o.Ledger = "ledger.json"
	if _, err := BuildRemedySummary(bg, o, 20); !errors.As(err, &r) || r.Code != "invalid-arguments" {
		t.Fatalf("ledger: %v", err)
	}
	if _, err := BuildRemedySummary(bg, diffOpts(root, base, c4), 0); !errors.As(err, &r) || r.Code != "invalid-arguments" {
		t.Fatalf("zero commits: %v", err)
	}
}

// CCE-V0-296 (#1934): a changed server controller or route declaration that no join placed names
// the endpoint join's inputs and no reach_exempt key; any other non-UI path keeps its exemption.
func TestCCEV0296ControllerRemedyNamesEndpointJoin(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitFiles(t, root, map[string]*string{"app/controllers/api/items_controller.rb": text("class ItemsController\nend\n"),
		"config/routes.rb": text("Rails.application.routes.draw do\nend\n"), "lib/tasks/x.rb": text("# x\n")}, "server")
	o := buildObligations(t, root, base, head)
	requireRemedies(t, o)
	for _, p := range []string{"app/controllers/api/items_controller.rb", "config/routes.rb"} {
		u := unknownWith(o, ReasonUnplacedPath, p)
		if u == nil || u.Remedy.Key != "" || !strings.Contains(u.Remedy.Uncertainty, "--endpoint-source") ||
			!strings.Contains(u.Remedy.Uncertainty, "only if no client calls it") {
			t.Fatalf("%s remedy: %s", p, o.Encoded())
		}
	}
	if u := unknownWith(o, ReasonUnplacedPath, "lib/tasks/x.rb"); u == nil || u.Remedy.Key != RemedyKeyReachExempt {
		t.Fatalf("non-UI remedy: %s", o.Encoded())
	}
	for p, want := range map[string]bool{"api/openapi.yaml": true, "swagger.json": true, "app/models/item.rb": false, "app/controllers/x.go": false, "config/routes/admin.rb": true} {
		if serverDeclaration(p) != want {
			t.Errorf("serverDeclaration(%s) != %v", p, want)
		}
	}
}

// CCE-V0-297 (#1934): every remediable merge lists its remedy set, every remedy counts the unknown
// spans it removes, and the greedy cover adds the fewest remedies for each further merge bounded.
func TestCCEV0297RemedyCover(t *testing.T) {
	id := func(s string) string { return DeclarationsPath + "#reach_exempt=" + s }
	merges := []RemedyMerge{
		{Commit: "c1", Unknown: 2, RemedySet: []string{id("a"), id("b")}},
		{Commit: "c2", Unknown: 1, RemedySet: []string{id("a")}},
		{Commit: "c3", Unknown: 3, RemedySet: []string{id("a"), id("b"), id("c")}},
		{Commit: "c4", Unknown: 1, Unremediable: true},
		{Commit: "c5", State: "COMPLETE"},
	}
	got := greedyCover(merges)
	want := []CoverStep{{Add: []string{id("a")}, Bounds: []string{"c2"}, Total: 1}, {Add: []string{id("b")}, Bounds: []string{"c1"}, Total: 2},
		{Add: []string{id("c")}, Bounds: []string{"c3"}, Total: 3}}
	if !slices.EqualFunc(got, want, func(a, b CoverStep) bool {
		return slices.Equal(a.Add, b.Add) && slices.Equal(a.Bounds, b.Bounds) && a.Total == b.Total
	}) {
		t.Fatalf("cover %+v", got)
	}

	root, base := fixtureRepo(t)
	c1 := commitFiles(t, root, map[string]*string{"server/a.go": text("package server\n"), "server/b.go": text("package server\n")}, "ab")
	c2 := commitFiles(t, root, map[string]*string{"server/a.go": text("package server // 2\n")}, "a2")
	raw, err := BuildRemedySummary(bg, diffOpts(root, base, c2), MaxRemedyCommits)
	var s RemedySummary
	if err != nil || json.Unmarshal(raw, &s) != nil {
		t.Fatalf("%v %s", err, raw)
	}
	if len(s.Merges) != 2 || len(s.Merges[0].RemedySet) != 2 || len(s.Cover) != 2 || s.Cover[0].Bounds[0] != c2 || s.Cover[1].Bounds[0] != c1 ||
		s.Remedies[0].Remedy.Candidates[0] != "server/a.go" || s.Remedies[0].Unknowns != 2 {
		t.Fatalf("summary %s", raw)
	}
}
