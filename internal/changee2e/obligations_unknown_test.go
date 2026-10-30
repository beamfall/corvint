package changee2e

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// requireClosedReasons fails when any unknown span carries a reason outside the CCE-V0-020 set.
func requireClosedReasons(t *testing.T, o *Obligations) {
	t.Helper()
	for _, u := range o.Unknown {
		if !slices.Contains(UnknownReasons, u.Reason) {
			t.Fatalf("unlisted reason %q: %s", u.Reason, o.Encoded())
		}
	}
}

func unknownWith(o *Obligations, reason, path string) *UnknownSpan {
	for i, u := range o.Unknown {
		if u.Reason == reason && u.Span.Path == path {
			return &o.Unknown[i]
		}
	}
	return nil
}

func buildObligations(t *testing.T, root, base, head string) *Obligations {
	t.Helper()
	o, err := BuildObligations(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	requireClosedReasons(t, o)
	return o
}

// CCE-V0-020: the unknown-reason set is closed and exactly the spec's list.
func TestCCEV0020ClosedUnknownReasonSet(t *testing.T) {
	want := []string{"unplaced-path", "screen-without-outcome", "map-unknown", "impact-truncated", "removed-at-head",
		"fan-out-over-limit", "blocks-not-declared"}
	if !slices.Equal(UnknownReasons, want) || len(reasonRank) != len(want) {
		t.Fatalf("reasons %v rank %v", UnknownReasons, reasonRank)
	}
	for _, r := range want {
		if _, ok := reasonRank[r]; !ok {
			t.Fatalf("%s has no rank", r)
		}
	}
}

// CCE-V0-019 (F1): a reached screen that no flow step expecting an outcome follows reads UNKNOWN
// with `screen-without-outcome`, never NONE.
func TestCCEV0019ReachedScreenWithoutOutcomeUnknown(t *testing.T) {
	root, _ := fixtureRepo(t)
	routes := strings.Replace(*overlayText(t, "app/routes.js"), "url: '/settings'",
		"url: '/settings',\n      templateUrl: 'app/components/settings.html'", 1)
	base := commitFiles(t, root, map[string]*string{"app/routes.js": &routes, "app/components/settings.html": text("<h1>Settings</h1>\n")}, "settings")
	head := commitFiles(t, root, map[string]*string{"app/components/settings.html": text("<h1>Club settings</h1>\n")}, "label")
	o := buildObligations(t, root, base, head)
	u := unknownWith(o, ReasonScreenWithoutOutcome, "app/components/settings.html")
	if o.State != Unknown || len(o.Obligations) != 0 || len(o.Unknown) != 1 || u == nil || u.Detail != "screen:admin:app.clubs.settings" ||
		u.Span.Start != 1 || u.Span.End != 1 {
		t.Fatalf("%s", o.Encoded())
	}
}

// CCE-V0-019 (F2): a map unknown in a changed file or on a reached screen reads UNKNOWN with
// `map-unknown` at the unknown's own line, beside the obligations the change still reaches.
func TestCCEV0019MapUnknownOnReachedScreenUnknown(t *testing.T) {
	root, base := fixtureRepo(t)
	routes := *overlayText(t, "app/routes.js") + "// reviewed\n"
	head := commitFiles(t, root, map[string]*string{"app/routes.js": &routes}, "routes")
	o := buildObligations(t, root, base, head)
	if o.State != Unknown || len(o.Obligations) == 0 {
		t.Fatalf("%s", o.Encoded())
	}
	lines := map[int]string{}
	for _, u := range o.Unknown {
		if u.Reason == ReasonMapUnknown && u.Span.Path == "app/routes.js" {
			lines[u.Span.Start] = u.Detail
		}
	}
	if lines[38] != "screen screen:admin:app.dynamic: non-literal-url" || lines[41] != "screen screen:admin:orphan.child: missing-parent" {
		t.Fatalf("map unknowns %v: %s", lines, o.Encoded())
	}
	// The one-screen diff touches neither a file nor a screen nor a flow with a map unknown.
	o = buildObligations(t, root, head, commitBehaviourChange(t, root))
	if o.State != Reached || len(o.Unknown) != 0 {
		t.Fatalf("unrelated map unknowns: %s", o.Encoded())
	}
}

// CCE-V0-019 (F3): any impact omission, whether it drops screens or only edges and spec files,
// makes every changed path UNKNOWN with `impact-truncated`, never NONE or a bare REACHED.
func TestCCEV0019ImpactTruncationUnknown(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	defer func(b int) { impactBudget = b }(impactBudget)
	for _, budget := range []int{600, 1200} {
		impactBudget = budget
		o := buildObligations(t, root, base, head)
		u := unknownWith(o, ReasonImpactTruncated, "app/components/teesheet.html")
		if o.State != Unknown || len(o.Unknown) != 1 || u == nil || !strings.HasPrefix(u.Detail, "impact omitted ") {
			t.Fatalf("budget %d: %s", budget, o.Encoded())
		}
	}
}

// CCE-V0-019 (F4): a flow or an outcome present at base and absent at head reads UNKNOWN with
// `removed-at-head` at its base anchor, never NONE.
func TestCCEV0019RemovedAtHeadUnknown(t *testing.T) {
	root, base := fixtureRepo(t)
	flow := "flows/book-tee-time.json"
	renamed := strings.ReplaceAll(*overlayTextFrom(t, "../appmap/testdata/fixture", flow), `"booked"`, `"confirmed"`)
	head := commitFiles(t, root, map[string]*string{flow: &renamed}, "rename outcome")
	o := buildObligations(t, root, base, head)
	u := unknownWith(o, ReasonRemovedAtHead, flow)
	if o.State != Unknown || u == nil || u.Detail != "outcome:book-tee-time/booked" {
		t.Fatalf("outcome removal: %s", o.Encoded())
	}
	gone := commitFiles(t, root, map[string]*string{flow: nil}, "drop flow")
	o = buildObligations(t, root, head, gone)
	u = unknownWith(o, ReasonRemovedAtHead, flow)
	if o.State != Unknown || len(o.Unknown) != 1 || u == nil || u.Detail != "flow:book-tee-time" {
		t.Fatalf("flow removal: %s", o.Encoded())
	}
}

// CCE-V0-019 (F7): obligations past the limit read UNKNOWN with `fan-out-over-limit` naming each
// dropped obligation, instead of refusing the document.
func TestCCEV0019FanOutOverLimitUnknown(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	defer func(n int) { maxObligations = n }(maxObligations)
	maxObligations = 0
	o := buildObligations(t, root, base, head)
	u := unknownWith(o, ReasonFanOutOverLimit, "app/components/teesheet.html")
	if o.State != Unknown || len(o.Obligations) != 0 || len(o.Unknown) != 1 || u == nil || u.Detail != "obligation:book-tee-time/booked" {
		t.Fatalf("%s", o.Encoded())
	}
}

// CCE-V0-019, CCE-V0-020: NONE only when every changed path is exempt by a base `reach_exempt`
// glob the report cites and the map has no unknowns; a diff cannot exempt itself, and map unknowns
// keep an otherwise empty report UNKNOWN.
func TestCCEV0020ReachExemptAndNone(t *testing.T) {
	root, _ := fixtureRepo(t)
	blocks := strings.Replace(*overlayText(t, DeclarationsPath), `"fixtures": [`,
		`"reach_exempt": [{"glob": "*.md", "reason": "documentation renders no screen"}],
  "fixtures": [`, 1)
	exemptBase := commitFiles(t, root, map[string]*string{DeclarationsPath: &blocks}, "exempt docs")
	readme := commitFiles(t, root, map[string]*string{"README.md": text("docs\n")}, "readme")
	o := buildObligations(t, root, exemptBase, readme)
	if o.State != Unknown || len(o.Exempt) != 1 || o.Exempt[0].Glob != "*.md" || len(o.Unknown) == 0 {
		t.Fatalf("map unknowns must keep an exempt-only diff UNKNOWN: %s", o.Encoded())
	}
	for _, u := range o.Unknown {
		if u.Reason != ReasonMapUnknown {
			t.Fatalf("%s", o.Encoded())
		}
	}
	// A map without unknowns: drop the ambiguous, dynamic and orphan states and the unresolved specs.
	routes := *overlayText(t, "app/routes.js")
	routes = strings.Replace(routes, "    .state('members2', {\n      parent: 'app.clubs',\n      url: '/members'\n    })\n", "", 1)
	routes = routes[:strings.Index(routes, "    .state('app.dynamic'")] + "    ;\n});\n"
	clean := commitFiles(t, root, map[string]*string{"app/routes.js": &routes, "e2e/specs/alias.spec.ts": nil,
		"e2e/specs/unresolved.spec.ts": nil}, "clean map")
	if cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: clean, Manifest: "appmap.json"}); err != nil || len(cat.m.Unknowns) != 0 {
		t.Fatalf("clean map: %v %+v", err, cat.m.Unknowns)
	}
	head := commitFiles(t, root, map[string]*string{"README.md": text("more docs\n")}, "readme 2")
	o = buildObligations(t, root, clean, head)
	if o.State != None || len(o.Unknown) != 0 || len(o.Obligations) != 0 || len(o.Exempt) != 1 ||
		o.Exempt[0] != (Exemption{Path: "README.md", Glob: "*.md", Reason: "documentation renders no screen"}) {
		t.Fatalf("exempt-only diff over a clean map: %s", o.Encoded())
	}
	// The exemption is read at base: a diff adding its own glob stays UNKNOWN.
	self := strings.Replace(blocks, `{"glob": "*.md"`, `{"glob": "server/*", "reason": "backend"}, {"glob": "*.md"`, 1)
	selfHead := commitFiles(t, root, map[string]*string{DeclarationsPath: &self, "server/handler.go": text("package server\n")}, "self exempt")
	o = buildObligations(t, root, head, selfHead)
	if o.State != Unknown || unknownWith(o, ReasonUnplacedPath, "server/handler.go") == nil {
		t.Fatalf("self exemption: %s", o.Encoded())
	}
}

// CCE-V0-003, CCE-V0-020: a malformed `reach_exempt` entry refuses `e2e-blocks-invalid`.
func TestCCEV0020ReachExemptInvalidRefuses(t *testing.T) {
	for _, entry := range []string{`{"glob": "[", "reason": "x"}`, `{"glob": "", "reason": "x"}`, `{"glob": "/abs", "reason": "x"}`,
		`{"glob": "*.md", "reason": ""}`, `{"glob": "*.md", "reason": "x", "extra": 1}`} {
		blocks := `{"schema": "e2e-block-declarations/0", "inputs": [], "fixtures": [], "reach_exempt": [` + entry + `]}`
		var r *Refusal
		if _, err := decodeDeclarations([]byte(blocks)); !errors.As(err, &r) || r.Code != "e2e-blocks-invalid" {
			t.Fatalf("%s: %v", entry, err)
		}
	}
}

// V1-1711: a relative --root, including the default ".", resolves to its absolute path for the
// catalog and obligations reads instead of reading the revision as not a commit.
func TestCCEV0086RelativeRootResolves(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	t.Chdir(root)
	if _, err := BuildCatalog(bg, CatalogOptions{Root: ".", Rev: head, Manifest: "appmap.json"}); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	o, err := BuildObligations(bg, DiffOptions{Root: ".", Base: base, Head: head, Manifest: "appmap.json"})
	if err != nil || o.State != Reached {
		t.Fatalf("obligations: %v", err)
	}
}
