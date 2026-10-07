package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// planRepo commits the issue-657 fixture, then the planner overlay (testdata/plan: two more admin
// flows and a second app), and returns the root, the revision and both maps built at it.
func planRepo(t *testing.T) (string, string, []*Map) {
	t.Helper()
	root, _ := fixtureRepo(t)
	err := filepath.WalkDir("testdata/plan", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel("testdata/plan", p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		writeFile(t, root, rel, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rev := commitAll(t, root, "planner overlay")
	admin := build(t, root, rev)
	market, err := Build(context.Background(), root, "marketplace.json", rev)
	if err != nil {
		t.Fatal(err)
	}
	return root, rev, []*Map{admin, market}
}

const fourSteps = "1) book a tee time; 2) check the slot status\n3. buy a gift card; change the club settings"

func planOf(t *testing.T, maps []*Map, steps []string, o PlanOptions) map[string]any {
	t.Helper()
	raw, err := Plan(context.Background(), maps, steps, o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"full-confidence"`) || strings.Contains(string(raw), `"confidence":"full"`) {
		t.Fatalf("plan reports full confidence:\n%s", raw)
	}
	return decode(t, raw)
}

func planSteps(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, s := range doc["steps"].([]any) {
		out = append(out, s.(map[string]any))
	}
	return out
}

func actions(step map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, a := range step["actions"].([]any) {
		out = append(out, a.(map[string]any))
	}
	return out
}

func explorationNeeds(step map[string]any) []string {
	out := []string{}
	for _, e := range step["exploration"].([]any) {
		out = append(out, e.(map[string]any)["need"].(string))
	}
	return out
}

// AMSP-V0-001: a request splits at `;` and line breaks with enumerators dropped; malformed input
// refuses before any lookup.
func TestAMSPV0001RequestSplitAndInputBounds(t *testing.T) {
	got := SplitRequest(fourSteps + ";;  \n(5) ")
	want := []string{"book a tee time", "check the slot status", "buy a gift card", "change the club settings"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitRequest = %q", got)
	}
	root, rev, maps := planRepo(t)
	o := PlanOptions{Options: Options{Root: root, Revision: rev}}
	many := make([]string, maxPlanSteps+1)
	for i := range many {
		many[i] = "book a tee time"
	}
	for name, tc := range map[string]struct {
		maps  []*Map
		steps []string
		o     PlanOptions
	}{
		"no maps":        {nil, want, o},
		"duplicate apps": {[]*Map{maps[0], maps[0]}, want, o},
		"no steps":       {maps, nil, o},
		"too many steps": {maps, many, o},
		"blank step":     {maps, []string{"  "}, o},
		"long step":      {maps, []string{strings.Repeat("a", maxPlanStepBytes+1)}, o},
		"control":        {maps, []string{"book\x1b[2J"}, o},
		"newline":        {maps, []string{"book\nslot"}, o},
		"bad utf8":       {maps, []string{"book \xff"}, o},
		"budget":         {maps, want, PlanOptions{Options: Options{Root: root, Budget: 10}}},
		"budget+full":    {maps, want, PlanOptions{Options: Options{Root: root, Budget: 4096, Full: true}}},
	} {
		if _, err := Plan(context.Background(), tc.maps, tc.steps, tc.o); codeOf(err) != "appmap-invalid-query" {
			t.Errorf("%s: err = %v, want appmap-invalid-query", name, err)
		}
	}
}

// AMSP-V0-002 AMSP-V0-003 AMSP-V0-004 AMSP-V0-006 AMSP-V0-009: on the issue-657 fixture a
// four-step, two-app request resolves every step, reuses one session per app, stays on the screen
// a previous step left, hands the route parameter it produced to a later goto, and is
// deterministic under its byte cap whatever the map order.
func TestAMSPV0002MultiStepPlanOnFixture(t *testing.T) {
	root, rev, maps := planRepo(t)
	o := PlanOptions{Options: Options{Root: root, Revision: rev}}
	steps := SplitRequest(fourSteps)
	raw, err := Plan(context.Background(), maps, steps, o)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Plan(context.Background(), []*Map{maps[1], maps[0]}, steps, o)
	if !bytes.Equal(raw, again) {
		t.Fatal("plan depends on map order or is not deterministic")
	}
	if len(raw) > DefaultPlanBudget {
		t.Fatalf("plan is %d bytes, over its %d cap", len(raw), DefaultPlanBudget)
	}
	doc := planOf(t, maps, steps, o)
	if doc["schema"] != PlanSchema || doc["status"] != "COMPLETE" || doc["authority"] != "candidate" || doc["evaluated_revision"] != rev {
		t.Fatalf("head: %v %v %v %v", doc["schema"], doc["status"], doc["authority"], doc["evaluated_revision"])
	}
	st := planSteps(t, doc)
	wantFlows := []string{"flow:book-tee-time", "flow:check-slot-status", "flow:buy-gift-card", "flow:edit-club-settings"}
	for i, s := range st {
		if s["status"] != StepMapped || s["flow"] != wantFlows[i] || s["confidence"] != "candidate" {
			t.Fatalf("step %d: %v %v %v", i+1, s["status"], s["flow"], s["confidence"])
		}
		if len(s["asserts"].([]any)) == 0 {
			t.Fatalf("step %d asserts no outcome", i+1)
		}
	}
	sessions := doc["sessions"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("sessions = %v", sessions)
	}
	admin := sessions[0].(map[string]any)
	if admin["app"] != "admin" || !reflect.DeepEqual(admin["steps"], []any{1.0, 2.0, 4.0}) {
		t.Fatalf("admin session = %v", admin)
	}
	nav := func(step int) []string {
		out := []string{}
		for _, a := range actions(st[step-1]) {
			out = append(out, a["navigation"].(string))
		}
		return out
	}
	for step, want := range map[int][]string{1: {"goto", "follow", "in-screen"}, 2: {"stay"}, 3: {"goto"}, 4: {"goto", "in-screen"}} {
		if got := nav(step); !reflect.DeepEqual(got, want) {
			t.Errorf("step %d navigation = %v, want %v", step, got, want)
		}
	}
	// A spec line reusing a selector is not a callable method.
	if a := actions(st[1])[0]; a["methods_total"] != 0.0 || len(a["methods"].([]any)) != 0 {
		t.Fatalf("non-method reuse counted: %v", a)
	}
	if !reflect.DeepEqual(st[0]["produces"], []any{"clubId"}) || !reflect.DeepEqual(st[3]["consumes"], []any{"clubId"}) {
		t.Fatalf("handoff produces=%v consumes=%v", st[0]["produces"], st[3]["consumes"])
	}
	handoff := doc["handoff"].([]any)
	if len(handoff) != 1 {
		t.Fatalf("handoff = %v", handoff)
	}
	h := handoff[0].(map[string]any)
	if h["source"] != "step" || h["producer_step"] != 1.0 || !reflect.DeepEqual(h["consumed_by"], []any{4.0}) {
		t.Fatalf("handoff = %v", h)
	}
	// The second step's precondition flow is satisfied by the first step.
	satisfied := false
	for _, p := range admin["setup"].(map[string]any)["preconditions"].([]any) {
		pv := p.(map[string]any)
		if pv["text"] == "flow:book-tee-time" && pv["satisfied_by_step"] == 1.0 && pv["status"] == "unverified" {
			satisfied = true
		}
	}
	if !satisfied {
		t.Fatalf("precondition flow not tied to step 1: %v", admin["setup"])
	}
	if st[0]["match"].(map[string]any)["coverage"] != 1.0 {
		t.Fatalf("match = %v", st[0]["match"])
	}
	// An explicit flow ID resolves without term matching.
	ex := planSteps(t, planOf(t, maps, []string{"flow:buy-gift-card"}, o))[0]
	if ex["flow"] != "flow:buy-gift-card" || ex["match"].(map[string]any)["basis"] != "explicit" {
		t.Fatalf("explicit = %v", ex)
	}
}

// AMSP-V0-005: a request no flow matches, one several flows match equally, and one whose flow has
// an unplaced step are UNMAPPED with the exploration each needs; a later step does not navigate
// as if an unmapped step ran.
func TestAMSPV0005UnmappedStepsFailClosed(t *testing.T) {
	root, rev, maps := planRepo(t)
	o := PlanOptions{Options: Options{Root: root, Revision: rev}}
	doc := planOf(t, maps, []string{"book a tee time", "delete the club", "slot", "view the member list", "check the slot status"}, o)
	if doc["status"] != "INCOMPLETE" || len(doc["gaps"].([]any)) != 3 {
		t.Fatalf("status %v gaps %v", doc["status"], doc["gaps"])
	}
	st := planSteps(t, doc)
	for i, want := range []struct{ status, reason, need string }{
		{StepMapped, "", ""},
		{StepUnmapped, "no-matching-flow", "flow-intent"},
		{StepUnmapped, "ambiguous-flow", "flow-intent"},
		{StepUnmapped, "unplaced-step", "navigation-step"},
		{StepMapped, "", ""},
	} {
		s := st[i]
		if s["status"] != want.status || (want.reason != "" && s["reason"] != want.reason) {
			t.Fatalf("step %d: %v %v", i+1, s["status"], s["reason"])
		}
		if want.need != "" {
			if s["confidence"] != "none" || !reflect.DeepEqual(explorationNeeds(s), []string{want.need}) {
				t.Fatalf("step %d: confidence %v exploration %v", i+1, s["confidence"], s["exploration"])
			}
		}
	}
	if len(st[2]["candidates"].([]any)) != 2 {
		t.Fatalf("ambiguous candidates = %v", st[2]["candidates"])
	}
	if a := actions(st[3]); len(a) != 1 || a[0]["reason"] != "ambiguous-template" {
		t.Fatalf("unplaced action = %v", a)
	}
	// Step 4 resolved to the admin app but is not composed, so step 5 cannot assume the page.
	if a := actions(st[4]); a[0]["navigation"] != "goto" {
		t.Fatalf("step after a gap navigated %v, want goto", a[0]["navigation"])
	}
}

// AMSP-V0-005: anchors changed after the map was built make the step STALE with a rebuild
// exploration; a git failure makes it UNKNOWN. Neither is reported as mapped.
func TestAMSPV0005StaleAndUnknownFreshness(t *testing.T) {
	root, _, maps := planRepo(t)
	page := filepath.Join(root, "e2e/pages/teesheet.page.ts")
	data, _ := os.ReadFile(page)
	writeFile(t, root, "e2e/pages/teesheet.page.ts", strings.Replace(string(data), "getByTestId('slot')", "getByTestId('slot-cell')", 1))
	head := commitAll(t, root, "page object drift")
	doc := planOf(t, maps, []string{"book a tee time", "check the slot status"}, PlanOptions{Options: Options{Root: root, Revision: head}})
	st := planSteps(t, doc)
	if st[0]["status"] != StepStale || st[0]["confidence"] != "stale" || !reflect.DeepEqual(explorationNeeds(st[0]), []string{"rebuild-map"}) {
		t.Fatalf("stale step = %v %v %v", st[0]["status"], st[0]["confidence"], st[0]["exploration"])
	}
	if st[1]["status"] != StepMapped || actions(st[1])[0]["navigation"] != "goto" {
		t.Fatalf("step after stale = %v %v", st[1]["status"], actions(st[1])[0]["navigation"])
	}
	if doc["status"] != "INCOMPLETE" {
		t.Fatalf("status = %v", doc["status"])
	}
	unknown := planSteps(t, planOf(t, maps, []string{"book a tee time"}, PlanOptions{Options: Options{Root: t.TempDir()}}))[0]
	if unknown["status"] != StepUnknown || unknown["confidence"] != "unknown" {
		t.Fatalf("no git: %v %v", unknown["status"], unknown["confidence"])
	}
}

// fakeVerifier is an Overlay that answers VerificationFactKind facts. status returns
// "VERIFIED@<rev>" for a VERIFIED fact at rev, any other text verbatim, or "" for no fact. Every
// ID also gets a VERIFIED fact of another kind, and an unasked ID gets one of the right kind; the
// planner must ignore both.
type fakeVerifier struct {
	calls  int
	ids    []string
	status func(id string) string
	err    error
}

func (f *fakeVerifier) Facts(_ context.Context, ids []string) ([]Fact, error) {
	f.calls++
	f.ids = append([]string{}, ids...)
	if f.err != nil {
		return nil, f.err
	}
	out := []Fact{}
	for _, id := range ids {
		out = append(out, Fact{ElementID: id, Source: "fake", Kind: "note", Text: "VERIFIED", Revision: strings.Repeat("c", 40)})
		s := f.status(id)
		if s == "" {
			continue
		}
		fact := Fact{ElementID: id, Source: "fake", Kind: VerificationFactKind, Text: s}
		if rev, ok := strings.CutPrefix(s, "VERIFIED@"); ok {
			fact.Text, fact.Revision = "VERIFIED", rev
		}
		out = append(out, fact)
	}
	out = append(out, Fact{ElementID: "step:not-asked/x", Kind: VerificationFactKind, Text: "VERIFIED", Revision: strings.Repeat("a", 40)})
	return out, nil
}

func verifiers(v ...*fakeVerifier) []Overlay {
	out := []Overlay{}
	for _, f := range v {
		out = append(out, f)
	}
	return out
}

// AMSP-V0-007: per-step verification comes only through VerificationFactKind overlay facts.
// Absent reads unverified, VERIFIED stands only at the evaluated revision, CONTRADICTED is a gap,
// conflicting facts resolve to the most restrictive, malformed or failed verification is
// reported, and the plan's authority stays candidate.
func TestAMSPV0007VerificationSeam(t *testing.T) {
	root, rev, maps := planRepo(t)
	steps := []string{"book a tee time"}
	run := func(v ...*fakeVerifier) (map[string]any, map[string]any) {
		doc := planOf(t, maps, steps, PlanOptions{Options: Options{Root: root, Revision: rev, Overlays: verifiers(v...)}})
		return doc, planSteps(t, doc)[0]
	}
	_, s := run()
	for _, a := range actions(s) {
		if a["verification"] != "unverified" {
			t.Fatalf("default verification = %v", a["verification"])
		}
	}
	all := &fakeVerifier{status: func(string) string { return "VERIFIED@" + rev }}
	doc, s := run(all)
	if all.calls != 1 || s["confidence"] != "run-verified" || doc["authority"] != "candidate" {
		t.Fatalf("calls %d confidence %v authority %v", all.calls, s["confidence"], doc["authority"])
	}
	if !reflect.DeepEqual(all.ids, uniqueSorted(all.ids)) || len(all.ids) == 0 {
		t.Fatalf("verifier ids not sorted/unique: %v", all.ids)
	}
	other := &fakeVerifier{status: func(string) string { return "VERIFIED@" + strings.Repeat("b", 40) }}
	_, s = run(other)
	if s["confidence"] != "candidate" || actions(s)[0]["verification"] != "UNVERIFIED_AT_HEAD" {
		t.Fatalf("other revision: %v %v", s["confidence"], actions(s)[0]["verification"])
	}
	contra := &fakeVerifier{status: func(id string) string {
		if id == "step:book-tee-time/select-slot" {
			return "CONTRADICTED"
		}
		return "VERIFIED@" + rev
	}}
	doc, s = run(contra)
	if s["status"] != StepContradicted || s["confidence"] != "contradicted" || doc["status"] != "INCOMPLETE" ||
		!reflect.DeepEqual(explorationNeeds(s), []string{"re-explore"}) {
		t.Fatalf("contradicted: %v %v %v", s["status"], s["confidence"], s["exploration"])
	}
	// Conflicting overlays: one CONTRADICTED fact defeats any number of VERIFIED ones, and one
	// unverified fact keeps the step at candidate.
	for _, c := range []struct{ status, confidence string }{{"CONTRADICTED", "contradicted"}, {"unverified", "candidate"}, {"UNVERIFIED_AT_HEAD", "candidate"}} {
		doc, s = run(all, &fakeVerifier{status: func(string) string { return c.status }})
		if s["confidence"] != c.confidence || doc["authority"] != "candidate" {
			t.Fatalf("conflict with %s: confidence %v", c.status, s["confidence"])
		}
	}
	for _, malformed := range []string{"yes", "VERIFIED@", "VERIFIED@" + strings.ToUpper(rev), "verified@" + rev} {
		bad := &fakeVerifier{status: func(string) string { return malformed }}
		doc, s = run(all, bad)
		if s["confidence"] != "candidate" || !strings.Contains(stringOf(doc["unknowns"]), "verification-invalid") {
			t.Fatalf("malformed %q: %v %v", malformed, s["confidence"], doc["unknowns"])
		}
	}
	failed := &fakeVerifier{err: errors.New("receipts unreadable")}
	doc, s = run(failed)
	if s["confidence"] != "candidate" || !strings.Contains(stringOf(doc["unknowns"]), "verification-unavailable") {
		t.Fatalf("failed: %v %v", s["confidence"], doc["unknowns"])
	}
}

func stringOf(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// AMSP-V0-008: the draft has one test.step per request step, each MAPPED step names its outcome
// assertions and reuses fresh page-object methods, the handoff is captured from the URL, and an
// UNMAPPED step throws instead of running.
func TestAMSPV0008DraftSkeleton(t *testing.T) {
	root, rev, maps := planRepo(t)
	steps := append(SplitRequest(fourSteps), "delete the club")
	doc := planOf(t, maps, steps, PlanOptions{Options: Options{Root: root, Revision: rev}, Draft: true})
	d := doc["draft"].(map[string]any)
	if p := d["proposed_path"].(string); !strings.HasPrefix(p, "e2e/specs/plan-") || !strings.HasSuffix(p, ".spec.ts") {
		t.Fatalf("proposed path %q", p)
	}
	lines := []string{}
	for _, l := range d["lines"].([]any) {
		lines = append(lines, l.(string))
	}
	src := strings.Join(lines, "\n")
	if n := strings.Count(src, "await test.step("); n != len(steps) {
		t.Fatalf("%d test.step blocks for %d steps:\n%s", n, len(steps), src)
	}
	for _, want := range []string{
		"TODO assert outcome booked:", "TODO assert outcome status-shown:", "TODO assert outcome purchased:", "TODO assert outcome saved:",
		"await adminTeeSheetPage.selectSlot();", "await marketplaceShopPage.buyGiftCard();",
		`params.set("admin:clubId", routeParam(adminPage.url(), /\/#!\/clubs\/([^/?#]+)\/teesheets(?:[?#]|$)/));`,
		"await adminPage.goto(`/#!/clubs/${param(\"admin:clubId\")}/settings`);",
		"await expect(adminPage).toHaveURL(",
		`throw new Error("UNMAPPED step 5: no-matching-flow");`,
		"test.fixme(true,",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("draft lacks %q:\n%s", want, src)
		}
	}
	imports := stringOf(d["imports"])
	for _, want := range []string{"TeeSheetPage", "HomePage", "ShopPage", "../../shop-e2e/pages/shop.page"} {
		if !strings.Contains(imports, want) {
			t.Errorf("imports lack %q: %s", want, imports)
		}
	}
	if _, ok := planOf(t, maps, steps, PlanOptions{Options: Options{Root: root, Revision: rev}})["draft"]; ok {
		t.Fatal("draft returned without being requested")
	}
}

// AMSP-V0-009: the plan head is never trimmed; a budget it does not fit refuses.
func TestAMSPV0009BudgetRefusesNotTruncates(t *testing.T) {
	root, rev, maps := planRepo(t)
	_, err := Plan(context.Background(), maps, SplitRequest(fourSteps), PlanOptions{Options: Options{Root: root, Revision: rev, Budget: MinBudget}})
	if codeOf(err) != "appmap-budget-too-small" {
		t.Fatalf("err = %v", err)
	}
}

// URL helpers escape template text for JavaScript.
func TestAMSPV0008URLHelpersEscape(t *testing.T) {
	if got := urlExpr("app", "/a`b$/{id}\\"); got != "`/a\\`b\\$/${param(\"app:id\")}\\\\`" {
		t.Fatalf("urlExpr = %s", got)
	}
	if got := urlRegex("/a.b/{id}/c", "id"); got != `/\/a\.b\/([^/?#]+)\/c(?:[?#]|$)/` {
		t.Fatalf("urlRegex = %s", got)
	}
	if got := jsIdent("my-app.2"); got != "myApp2" {
		t.Fatalf("jsIdent = %s", got)
	}
}

// AMSP-V0-008: a reused method that takes parameters is not called without arguments; the draft
// falls back to the step's locator and says why.
func TestAMSPV0008MethodWithArgumentsNotCalled(t *testing.T) {
	root, _, _ := planRepo(t)
	page := "e2e/pages/teesheet.page.ts"
	data, _ := os.ReadFile(filepath.Join(root, page))
	writeFile(t, root, page, strings.Replace(string(data), "async book() {", "async book(slot: string) {", 1))
	rev := commitAll(t, root, "book takes a slot")
	doc := planOf(t, []*Map{build(t, root, rev)}, []string{"book a tee time"}, PlanOptions{Options: Options{Root: root, Revision: rev}, Draft: true})
	src := stringOf(doc["draft"].(map[string]any)["lines"])
	if strings.Contains(src, "adminTeeSheetPage.book(") || !strings.Contains(src, "adminTeeSheetPage.selectSlot();") ||
		!strings.Contains(src, "is not a public method callable without arguments; not called") {
		t.Fatalf("draft: %s", src)
	}
}

// AMSP-V0-006, AMSP-V0-008: two apps with a route parameter of the same name keep separate values
// in the draft; handoff never crosses apps.
func TestAMSPV0006HandoffStaysInItsApp(t *testing.T) {
	root, _, _ := planRepo(t)
	data, _ := os.ReadFile(filepath.Join(root, "shop/routes.js"))
	writeFile(t, root, "shop/routes.js", strings.Replace(string(data), "url: '/shop'", "url: '/clubs/:clubId/shop'", 1))
	flow := filepath.Join(root, "shop/flows/buy-gift-card.json")
	data, _ = os.ReadFile(flow)
	writeFile(t, root, "shop/flows/buy-gift-card.json", strings.Replace(string(data), `"state": "/shop"`, `"state": "/clubs/{clubId}/shop"`, 1))
	rev := commitAll(t, root, "shop under a club")
	market, err := Build(context.Background(), root, "marketplace.json", rev)
	if err != nil {
		t.Fatal(err)
	}
	doc := planOf(t, []*Map{build(t, root, rev), market}, SplitRequest(fourSteps), PlanOptions{Options: Options{Root: root, Revision: rev}, Draft: true})
	src := stringOf(doc["draft"].(map[string]any)["lines"])
	for _, want := range []string{`params.set(\"marketplace:clubId\", \"TODO\")`, `param(\"marketplace:clubId\")`, `param(\"admin:clubId\")`} {
		if !strings.Contains(src, want) {
			t.Errorf("draft lacks %s: %s", want, src)
		}
	}
	if strings.Contains(src, `param(\"clubId\")`) || strings.Contains(src, `params.set(\"clubId\"`) {
		t.Fatalf("unscoped parameter: %s", src)
	}
	for _, h := range doc["handoff"].([]any) {
		if hv := h.(map[string]any); hv["app"] == "marketplace" && (hv["source"] != "setup" || stringOf(hv["consumed_by"]) != "[3]") {
			t.Fatalf("marketplace handoff = %v", hv)
		}
	}
}

// AMSP-V0-007: an unverified selector keeps its step at candidate even when the step and its
// methods are verified.
func TestAMSPV0007UnverifiedSelectorBlocksRunVerified(t *testing.T) {
	root, rev, maps := planRepo(t)
	st := planSteps(t, planOf(t, maps, []string{"buy a gift card"}, PlanOptions{Options: Options{Root: root, Revision: rev}}))[0]
	all := map[string]string{}
	for _, av := range actions(st) {
		all[av["step"].(string)] = "VERIFIED@" + rev
		for _, me := range av["methods"].([]any) {
			all[me.(map[string]any)["id"].(string)] = "VERIFIED@" + rev
		}
	}
	got := planSteps(t, planOf(t, maps, []string{"buy a gift card"}, PlanOptions{Options: Options{Root: root, Revision: rev, Overlays: verifiers(&fakeVerifier{status: func(id string) string { return all[id] }})}}))[0]
	if got["confidence"] != "candidate" {
		t.Fatalf("confidence with unverified selector = %v", got["confidence"])
	}
	sel := actions(st)[0]["selector"].(map[string]any)["id"].(string)
	all[sel] = "VERIFIED@" + rev
	got = planSteps(t, planOf(t, maps, []string{"buy a gift card"}, PlanOptions{Options: Options{Root: root, Revision: rev, Overlays: verifiers(&fakeVerifier{status: func(id string) string { return all[id] }})}}))[0]
	if got["confidence"] != "run-verified" {
		t.Fatalf("confidence with every element verified = %v", got["confidence"])
	}
}

// AMSP-V0-008: a page-object class the suite never imports by name is not imported or called.
func TestAMSPV0008DefaultExportNotNamedImport(t *testing.T) {
	root, _, _ := planRepo(t)
	for _, p := range []string{"shop-e2e/pages/shop.page.ts", "shop-e2e/specs/buy.spec.ts"} {
		data, _ := os.ReadFile(filepath.Join(root, p))
		s := strings.Replace(string(data), "export class ShopPage", "export default class ShopPage", 1)
		writeFile(t, root, p, strings.Replace(s, "import { ShopPage }", "import ShopPage", 1))
	}
	rev := commitAll(t, root, "default export")
	market, err := Build(context.Background(), root, "marketplace.json", rev)
	if err != nil {
		t.Fatal(err)
	}
	doc := planOf(t, []*Map{market}, []string{"buy a gift card"}, PlanOptions{Options: Options{Root: root, Revision: rev}, Draft: true})
	d := doc["draft"].(map[string]any)
	src, imports := stringOf(d["lines"]), stringOf(d["imports"])
	if strings.Contains(src, "buyGiftCard()") || strings.Contains(src, "new ShopPage") || strings.Contains(imports, "import { ShopPage } from") ||
		!strings.Contains(imports, "UNRESOLVED import { ShopPage }: no named import") || !strings.Contains(src, "getByTestId(") {
		t.Fatalf("draft: %s %s", imports, src)
	}
}
