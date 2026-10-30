package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const skHowExtra = `import { test, expect } from '@playwright/test';
test.use({ storageState: 'e2e/.auth/user.json' });
test('archives an invoice', async ({ page }) => {
  await page.goto('/invoices/2');
  await archiveIt(page);
  await page.goto('/invoices');
  await expect(page.getByText('Archived')).toBeVisible();
});
`

type skHowDoc struct {
	Schema     string      `json:"schema"`
	Status     string      `json:"status"`
	Freshness  string      `json:"freshness"`
	Label      string      `json:"label"`
	Gaps       []HowGap    `json:"gaps"`
	GoalScreen *HowScreen  `json:"goal_screen"`
	Oracle     []HowOracle `json:"oracle"`
	Candidate  *struct {
		ID   string `json:"id"`
		Test string `json:"test"`
	} `json:"candidate"`
	Setup      []HowSetup       `json:"setup"`
	Conditions []SetupCondition `json:"conditions"`
	Steps      []HowStep        `json:"steps"`
	Path       []HowHop         `json:"path"`
	Candidates []candidateView  `json:"candidates"`
}

func skHowFixture(t *testing.T) (string, *Map) {
	t.Helper()
	return railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		delete(files, "e2e/specs/orders.spec.ts")
		files["e2e/specs/flows.spec.ts"] = skFlowSpecs
		files["e2e/specs/extra.spec.ts"] = skHowExtra
		files["e2e/auth/login.setup.ts"] = "import { test as setup } from '@playwright/test';\nsetup('login', async ({ page }) => {\n" +
			"  await page.context().storageState({ path: 'e2e/.auth/user.json' });\n});\n"
		files["app/views/orders/index.html.erb"] = "<%= link_to \"Open\", order_path(@order) %>\n"
		files["flows/retired.json"] = `{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":[]}` + "\n"
	})
}

func skHow(t *testing.T, root string, m *Map, goal string, conditions []string, ledger *EdgeLedger) (skHowDoc, []byte) {
	t.Helper()
	o := Options{Root: root, Full: true}
	raw, err := How(context.Background(), m, []string{"e2e", "app/views"}, goal, conditions, nil, ledger, o)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := How(context.Background(), m, []string{"e2e", "app/views"}, goal, conditions, nil, ledger, o); !bytes.Equal(raw, again) {
		t.Fatal("how is not byte-stable")
	}
	var doc skHowDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	return doc, raw
}

func skGapCodes(gaps []HowGap) []string {
	out := []string{}
	for _, g := range gaps {
		out = append(out, g.Code)
	}
	return out
}

// SKH-V0-143 SKH-V0-145 SKH-V0-146 SKH-V0-148 SKH-V0-150: a goal resolves to one candidate flow;
// the answer names the goal screen, the primary test's steps on their screens, each hop with its
// admitted edges and their provenance, and the oracle the variation's sources share, every element
// anchored; the answer is a labelled proposal, byte-stable, and the read leaves the repo unchanged.
func TestSKHV0_143_HowAnswersGoal(t *testing.T) {
	root, m := skHowFixture(t)
	before := skRepoState(t, root)
	doc, raw := skHow(t, root, m, "ship an order", nil, nil)
	if doc.Schema != HowSchema || doc.Status != HowComplete || doc.Freshness != Fresh || len(doc.Gaps) != 0 || !strings.Contains(doc.Label, "not accepted intent") {
		t.Fatalf("answer = %s", raw)
	}
	if doc.GoalScreen == nil || doc.GoalScreen.URL != "/orders/{id}" || doc.GoalScreen.Anchor.Path != "config/routes.rb" {
		t.Fatalf("goal screen = %+v", doc.GoalScreen)
	}
	if doc.Candidate == nil || !strings.HasPrefix(doc.Candidate.Test, "test:e2e/specs/flows.spec.ts#") {
		t.Fatalf("candidate = %s", raw)
	}
	if len(doc.Steps) != 3 || doc.Steps[0].Kind != StepVisit || doc.Steps[1].Kind != StepClick || doc.Steps[1].Screen != doc.Steps[0].Screen {
		t.Fatalf("steps = %+v", doc.Steps)
	}
	for _, s := range doc.Steps {
		if s.Anchor.Path != "e2e/specs/flows.spec.ts" || s.Anchor.Start == 0 {
			t.Fatalf("unanchored step %+v", s)
		}
	}
	if len(doc.Path) != 1 || doc.Path[0].To != doc.GoalScreen.ID || doc.Path[0].Basis != "edge" {
		t.Fatalf("path = %+v", doc.Path)
	}
	static := false
	for _, e := range doc.Path[0].Edges {
		if e.Anchor == nil {
			t.Fatalf("unanchored edge %+v", e)
		}
		static = static || e.Provenance == ProvenanceStatic && e.Anchor.Path == "app/views/orders/index.html.erb"
	}
	if !static {
		t.Fatalf("static link not admitted: %+v", doc.Path[0].Edges)
	}
	if len(doc.Oracle) != 1 || doc.Oracle[0].Locator != "text=Shipped" || doc.Oracle[0].Matcher != "toBeVisible" || doc.Oracle[0].Anchor.Path != "e2e/specs/flows.spec.ts" {
		t.Fatalf("oracle = %+v", doc.Oracle)
	}
	if after := skRepoState(t, root); after != before {
		t.Fatal("how changed the repository")
	}
}

// SKH-V0-144 SKH-V0-149: a goal no candidate covers abstains with no-route, and a tie for the best
// names every tied candidate as ambiguous-goal; neither returns a path.
func TestSKHV0_144_HowAbstains(t *testing.T) {
	root, m := skHowFixture(t)
	doc, raw := skHow(t, root, m, "launch the rocket", nil, nil)
	if doc.Status != HowAbstained || len(doc.Gaps) != 1 || doc.Gaps[0].Code != GapNoRoute || doc.Candidate != nil || len(doc.Steps) != 0 {
		t.Fatalf("no-route = %s", raw)
	}
	doc, raw = skHow(t, root, m, "click", nil, nil)
	if doc.Status != HowAbstained || len(doc.Gaps) != 1 || doc.Gaps[0].Code != GapAmbiguousGoal || len(doc.Candidates) < 2 || doc.Candidates[0].Coverage != doc.Candidates[1].Coverage {
		t.Fatalf("ambiguous = %s", raw)
	}
}

// SKH-V0-147 SKH-V0-144: the setup that arranges the primary test is listed with its anchor; a
// --condition no record produces is a no-setup-for-precondition gap; an unresolved helper call is
// a no-page-object gap; and the answer is INCOMPLETE, never COMPLETE.
func TestSKHV0_147_HowSetupAndGaps(t *testing.T) {
	root, m := skHowFixture(t)
	doc, raw := skHow(t, root, m, "archive an invoice", []string{"e2e/.auth/user.json", "a refund exists"}, nil)
	if doc.Status != HowIncomplete {
		t.Fatalf("status = %s", raw)
	}
	codes := strings.Join(skGapCodes(doc.Gaps), ",")
	if codes != GapNoPageObject+","+GapNoSetup {
		t.Fatalf("gaps = %s", raw)
	}
	state := false
	for _, s := range doc.Setup {
		state = state || s.Kind == SetupStorageState && s.Target == "e2e/.auth/user.json" && s.Anchor.Path == "e2e/specs/extra.spec.ts"
	}
	if !state {
		t.Fatalf("setup = %+v", doc.Setup)
	}
	if len(doc.Conditions) != 2 || doc.Conditions[0].Status != ConditionCandidates || doc.Conditions[1].Status != ConditionUnresolved {
		t.Fatalf("conditions = %+v", doc.Conditions)
	}
	if doc.Path[0].Basis != "visit" || len(doc.Path[0].Edges) != 0 {
		t.Fatalf("a hop with no admitted edge = %+v", doc.Path)
	}
}

// SKH-V0-146: a PROVEN row of an untrimmed ledger built from the map is admitted with ledger
// provenance and the receipt's test; a ledger whose digest does not match its content is refused.
func TestSKHV0_146_HowAdmitsLedgerEdges(t *testing.T) {
	root, m := skHowFixture(t)
	first, _ := skHow(t, root, m, "pay an invoice", nil, nil)
	if len(first.Path) != 1 {
		t.Fatalf("path = %+v", first.Path)
	}
	hop := first.Path[0]
	l := EdgeLedger{Schema: EdgeLedgerSchema, App: m.App, Revision: m.Revision, MapDigest: m.Digest, Authority: AuthorityLearned,
		Edges: []LedgerEdge{
			{ID: "edge-proven", From: hop.From, To: hop.To, Trigger: "click", Status: EdgeProven, RedundantCrossers: []string{},
				Evidence: &EdgeCrossing{Flow: "pay", Verification: StepVerification{Status: Verified, Receipt: &ReceiptRef{TestKey: "invoices.spec.ts>pays", Profile: "p"}, Authority: AuthorityLearned}}},
			{ID: "edge-unproven", From: hop.From, To: hop.To, Trigger: "click", Status: "UNPROVEN", RedundantCrossers: []string{}},
		},
		Unsatisfied: []string{}, Omitted: map[string]int{"edges": 0}}
	complete, _ := json.Marshal(l)
	l.Digest = digest(complete)
	doc, raw := skHow(t, root, m, "pay an invoice", nil, &l)
	ledger := []HowEdge{}
	for _, e := range doc.Path[0].Edges {
		if e.Provenance == ProvenanceLedger {
			ledger = append(ledger, e)
		}
	}
	if len(ledger) != 1 || ledger[0].ID != "edge-proven" || ledger[0].Test != "invoices.spec.ts>pays" || ledger[0].Ledger != l.Digest || doc.Path[0].Basis != "edge" {
		t.Fatalf("ledger edges = %s", raw)
	}
	l.Edges[1].Status = EdgeProven
	_, err := How(context.Background(), m, []string{"e2e"}, "pay an invoice", nil, nil, &l, Options{Root: root, Full: true})
	var coded *gokernel.Error
	if !errors.As(err, &coded) || coded.Code != "appmap-invalid-query" {
		t.Fatalf("tampered ledger = %v", err)
	}
}

// SKH-V0-063 SKH-V0-151: a cited span that changed after the map revision makes the answer STALE
// with a stale-evidence gap, never COMPLETE.
func TestSKHV0_151_HowStale(t *testing.T) {
	root, m := skHowFixture(t)
	writeFile(t, root, "e2e/specs/flows.spec.ts", strings.ReplaceAll(skFlowSpecs, "await page.getByRole('button', { name: 'Pay' }).click();", "await page.getByRole('button', { name: 'Settle' }).click();"))
	commitAll(t, root, "edit")
	doc, raw := skHow(t, root, m, "pay an invoice", nil, nil)
	if doc.Status != HowStale || doc.Freshness != Stale || !strings.Contains(strings.Join(skGapCodes(doc.Gaps), ","), GapStaleEvidence) {
		t.Fatalf("stale = %s", raw)
	}
}

// SKH-V0-064 SKH-V0-152: the answer fits the byte budget, keeps its head, and refuses a goal or
// condition outside the bounds.
func TestSKHV0_152_HowBoundsAndBudget(t *testing.T) {
	root, m := skHowFixture(t)
	raw, err := How(context.Background(), m, []string{"e2e", "app/views"}, "ship an order", nil, nil, nil, Options{Root: root, Budget: 2600})
	if err != nil {
		var coded *gokernel.Error
		if !errors.As(err, &coded) || coded.Code != "appmap-budget-too-small" {
			t.Fatal(err)
		}
	} else if len(raw) > 2600 || !bytes.Contains(raw, []byte(`"status":"COMPLETE"`)) || !bytes.Contains(raw, []byte(`"omitted":`)) {
		t.Fatalf("budgeted answer (%d bytes) = %s", len(raw), raw)
	}
	for _, goal := range []string{"", " ", "a\nb", strings.Repeat("x", 257), "token ghp_" + strings.Repeat("a", 36)} {
		if _, err := How(context.Background(), m, []string{"e2e"}, goal, nil, nil, nil, Options{Root: root}); err == nil {
			t.Errorf("goal %q accepted", goal)
		}
	}
	if _, err := How(context.Background(), m, []string{"e2e"}, "ship", []string{"x\ny"}, nil, nil, Options{Root: root}); err == nil {
		t.Error("multi-line condition accepted")
	}
}

// SKH-V0-185 (V1-2641): a step run through a page-object method whose page object is joined to a
// screen (a manifest page_object_screens entry or the page-object-url join) runs on that screen;
// a change of screen there adds a hop (basis `page-object`, or `edge` as here when an admitted
// edge joins the screens), and the candidate's end screen, so the goal screen, follows it.
func TestSKHV0_185_PageObjectStepScreens(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "e2e/specs/booked.spec.ts", `import { test, expect } from '@playwright/test';
import { HomePage } from '../pages/home.page';
import { TeeSheetPage } from '../pages/teesheet.page';

test('shows the booked status', async ({ page }) => {
  await page.goto('/#!/home');
  const home = new HomePage(page);
  await home.goToTeeSheets();
  const sheet = new TeeSheetPage(page);
  await sheet.selectSlot();
  await sheet.book();
  await expect(page.locator('.slot-status')).toHaveText('Booked');
});
`)
	m := build(t, root, commitAll(t, root, "booked"))
	raw, err := How(context.Background(), m, []string{"e2e"}, "show the booked status", nil, nil, nil, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc skHowDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	home, sheet := screenID("admin", "app.home"), screenID("admin", "app.clubs.teesheets")
	if doc.Candidate == nil || !strings.HasPrefix(doc.Candidate.Test, "test:e2e/specs/booked.spec.ts#") || len(doc.Steps) != 4 {
		t.Fatalf("answer = %s", raw)
	}
	want := []string{home, home, sheet, sheet}
	for i, s := range doc.Steps {
		if s.Screen != want[i] {
			t.Errorf("step %d %s via %s: screen %s, want %s", s.Index, s.Action, s.Via, s.Screen, want[i])
		}
	}
	if len(doc.Path) != 1 || doc.Path[0].From != home || doc.Path[0].To != sheet || doc.Path[0].Basis != "edge" || len(doc.Path[0].Edges) == 0 || doc.Path[0].At != 3 {
		t.Errorf("path = %+v", doc.Path)
	}
	if doc.GoalScreen == nil || doc.GoalScreen.ID != sheet {
		t.Errorf("goal screen = %+v, want %s", doc.GoalScreen, sheet)
	}
	// Only the innermost via names the screen: an unjoined inner page object never borrows its
	// caller's screen.
	k := &SKModel{PageObjects: []SKPageObject{
		{ID: "function:e2e/po.ts#Outer", Join: StatusResolved, Screen: "s:outer"},
		{ID: "function:e2e/po.ts#Inner", Join: StatusUnknown},
	}}
	for via, want := range map[string]string{"function:e2e/po.ts#Outer.go": "s:outer", "function:e2e/po.ts#Outer.go function:e2e/po.ts#Inner.click": ""} {
		if got := k.stepScreen(SKStep{Kind: StepClick, Via: strings.Fields(via)}); got != want {
			t.Errorf("stepScreen(%s) = %q, want %q", via, got, want)
		}
	}
}

// SKH-V0-188 (V1-2650): a term-matched candidate that lacks a goal term another candidate carries,
// weighing at least as much as the lightest term it carries, is refused with no-route though its
// coverage clears minCoverage. The core-history shape: "open the manage area" chose a viewer flow
// on "open" and the common "area" while other candidates carried "manage". A lacked term no
// candidate uses keeps the choice but returns the term for a no-route gap, and a lacked term
// lighter than every carried one changes nothing.
func TestSKHV0_188_HowRefusesAnOutweighedMatch(t *testing.T) {
	m := &Map{App: "app"}
	flow := func(id string, ts ...string) *planFlow {
		pf := &planFlow{m: m, f: &Flow{ID: id}, terms: map[string]bool{}}
		for _, t := range ts {
			pf.terms[t] = true
		}
		return pf
	}
	flows := []*planFlow{flow("viewer", "open", "area"), flow("bulk", "manage", "area", "bulk"),
		flow("tags", "manage", "area", "tag"), flow("roots", "manage", "area", "root"), flow("setup", "open", "setup")}
	df := map[string]int{}
	for _, pf := range flows {
		for t := range pf.terms {
			df[t]++
		}
	}
	idf := func(t string) float64 { return math.Log(1 + float64(len(flows))/float64(max(df[t], 1))) }
	for _, c := range []struct{ goal, chosen, refused, unknown string }{
		{"open the manage area", "viewer", "unmatched-goal-term: manage", ""},
		{"open setup quickly", "setup", "", "quickly"},
		{"open setup area", "setup", "", ""},
		{"manage bulk area", "bulk", "", ""},
	} {
		res := resolve(c.goal, flows, idf)
		if res.flow == nil || res.flow.f.ID != c.chosen {
			t.Fatalf("%q: coverage alone chose %+v, want %s", c.goal, res, c.chosen)
		}
		got, unknown := outweighed(res, c.goal, flows, idf)
		switch {
		case strings.Join(unknown, " ") != c.unknown:
			t.Errorf("%q: unknown terms %q, want %q", c.goal, unknown, c.unknown)
		case c.refused == "" && got.flow != res.flow:
			t.Errorf("%q: refused %+v", c.goal, got)
		case c.refused != "" && (got.flow != nil || got.reason != c.refused || len(got.candidates) != len(res.candidates)):
			t.Errorf("%q: = %+v, want refused %q with the candidates kept", c.goal, got, c.refused)
		}
	}
	if explicit, _ := outweighed(resolution{flow: flows[0], match: matchView{Basis: "explicit"}}, "open the manage area", flows, idf); explicit.flow != flows[0] {
		t.Error("an explicit flow: goal was refused")
	}

	root, fm := skHowFixture(t)
	doc, raw := skHow(t, root, fm, "ship an order", nil, nil)
	if doc.Status != HowComplete {
		t.Fatalf("baseline = %s", raw)
	}
	doc, raw = skHow(t, root, fm, "ship an order overnight", nil, nil)
	if doc.Status != HowIncomplete || len(doc.Gaps) != 1 || doc.Gaps[0].Code != GapNoRoute || doc.Gaps[0].Detail != "unmatched-goal-term: overnight" ||
		doc.Candidate == nil || len(doc.Steps) == 0 {
		t.Fatalf("unknown goal term = %s", raw)
	}
	_, kf, _, _, _, err := inferRead(context.Background(), fm, []string{"e2e", "app/views"}, nil, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	pfs, fidf := howFlows(fm, kf)
	if resolve("ship an order overnight", pfs, fidf).flow == nil {
		t.Fatal("the fixture goal no longer clears minCoverage on coverage alone, so it proves nothing")
	}
}
