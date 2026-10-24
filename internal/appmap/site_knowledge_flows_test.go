package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

const skFlowSpecs = `import { test, expect } from '@playwright/test';
test('ships an order', async ({ page }) => {
  await page.goto('/orders');
  await page.getByRole('button', { name: 'Ship' }).click();
  await page.goto('/orders/1');
  await expect(page.getByText('Shipped')).toBeVisible();
});
test('ships another order', async ({ page }) => {
  await page.goto('/orders');
  await page.getByRole('button', { name: 'Ship' }).click();
  await page.goto('/orders/1');
  await expect(page.getByText('Shipped')).toBeVisible();
});
test('ships with a note', async ({ page }) => {
  await page.goto('/orders');
  await page.getByLabel('Note').fill('fragile');
  await page.getByRole('button', { name: 'Ship' }).click();
  await page.goto('/orders/1');
  await expect(page.getByText('Shipped')).toBeVisible();
  await expect(page.getByText('fragile')).toBeVisible();
});
test('pays an invoice', async ({ page }) => {
  await page.goto('/invoices');
  await page.getByRole('button', { name: 'Pay' }).click();
  await page.goto('/invoices/2');
  await expect(page.getByText('Paid')).toBeVisible();
});
test('pays again', async ({ page }) => {
  await page.goto('/invoices');
  await page.getByRole('button', { name: 'Pay' }).click();
  await page.goto('/invoices/2');
  await expect(page.getByText('Paid')).toBeVisible();
});
test('status page', async ({ page }) => {
  await page.goto('/status');
  await expect(page.getByText('ok')).toBeVisible();
});
test('no visit', async ({ page }) => {
  await page.getByRole('button', { name: 'Ship' }).click();
});
test('reports', async ({ page }) => {
  await page.goto('/reports');
  await page.mouse.wheel(0, 100);
});
`

// skFlowsFixture is the Rails resource fixture with one Playwright spec of the tests above.
func skFlowsFixture(t *testing.T) (string, *Map) {
	t.Helper()
	return railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		delete(files, "e2e/specs/orders.spec.ts")
		files["e2e/specs/flows.spec.ts"] = skFlowSpecs
		files["flows/retired.json"] = `{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":[]}` + "\n"
	})
}

// skTrustFor joins one test-knowledge record to the test titled title in the fixture spec.
func skTrustFor(t *testing.T, id, title, trust, verdict, selection, gap string) TestTrust {
	t.Helper()
	for i, line := range strings.Split(skFlowSpecs, "\n") {
		if strings.Contains(line, "test('"+title+"'") {
			return TestTrust{ID: id, Path: "e2e/specs/flows.spec.ts", Start: i + 1, End: i + 2, TrustLevel: trust, Verdict: verdict, Selection: selection, Gap: gap}
		}
	}
	t.Fatalf("no test %q", title)
	return TestTrust{}
}

type skFlowsDoc struct {
	Schema      string            `json:"schema"`
	Status      map[string]int    `json:"candidate_status"`
	Candidates  []SKCandidateFlow `json:"candidates"`
	Unclustered []SKUnclustered   `json:"unclustered"`
}

func skRepoState(t *testing.T, root string) string {
	t.Helper()
	out := []string{}
	for _, args := range [][]string{{"status", "--porcelain", "--ignored", "--untracked-files=all"}, {"show-ref"}, {"rev-parse", "HEAD"}, {"ls-files", "--stage"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		out = append(out, string(b))
	}
	return strings.Join(out, "\x00")
}

// SKH-V0-050 SKH-V0-051 SKH-V0-052 SKH-V0-053 SKH-V0-054 SKH-V0-120 SKH-V0-121 SKH-V0-122
// SKH-V0-123 SKH-V0-124 SKH-V0-125: tests cluster by start and end screen into candidate flows
// and by action sequence into variations; outcomes are the assertions every source makes; a
// run+reviewed candidate outranks better-supported unreviewed tests as the primary; a joined
// defect blocks its flow even at run+reviewed; an unrelated test is never a source; partial gaps
// are carried; conflicts and single-source candidates are named; inference leaves the worktree,
// index, refs and .corvint/ unchanged; and the emitted request, imported by flows import, yields
// proposed intents equal to the proposed candidates.
func TestSKHV0_050_054_FlowInferenceTrustAndImport(t *testing.T) {
	root, m := skFlowsFixture(t)
	trust := []TestTrust{
		skTrustFor(t, "tk:note", "ships with a note", "run+reviewed", "partial", "candidate", "the note text is not asserted after reload"),
		skTrustFor(t, "tk:pay", "pays an invoice", "run+reviewed", "defect", "blocks", ""),
		skTrustFor(t, "tk:status", "status page", "read-only", "unrelated", "excluded", ""),
	}
	before := skRepoState(t, root)
	o := Options{Root: root, Full: true}
	raw, err := InferFlows(context.Background(), m, []string{"e2e"}, trust, o)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := InferFlows(context.Background(), m, []string{"e2e"}, trust, o); !bytes.Equal(raw, again) {
		t.Fatal("infer rerun differs (SKH-V0-007)")
	}
	var doc skFlowsDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	id := func(state string) string { return screenID(m.App, state) }
	byStart := map[string]SKCandidateFlow{}
	for _, f := range doc.Candidates {
		if f.Authority != "learned" || f.Anchor.Blob == "" {
			t.Fatalf("candidate not learned or not anchored: %+v (SKH-V0-001, SKH-V0-003)", f)
		}
		byStart[f.Start] = f
	}
	ship := byStart[id("orders")]
	if ship.Status != CandidateProposed || ship.End != id("order") || ship.Support != 3 || ship.SingleSource || len(ship.Variations) != 2 {
		t.Fatalf("ship flow = %+v (SKH-V0-050, SKH-V0-121)", ship)
	}
	primary := ship.Variations[0]
	if primary.Support != 1 || primary.Sources[0].Title != "ships with a note" || primary.Sources[0].Rank != "candidate" ||
		primary.Sources[0].TrustLevel != "run+reviewed" || ship.Variations[1].Sources[0].Rank != "unreviewed" {
		t.Fatalf("primary variation = %+v (SKH-V0-052, SKH-V0-123)", ship.Variations)
	}
	actions := []string{}
	for _, s := range ship.Steps {
		actions = append(actions, s.Action)
	}
	if !reflect.DeepEqual(actions, []string{"visit /orders", "fill label=Note", "click role=button name=Ship", "visit /orders/1"}) {
		t.Fatalf("flow steps = %v (SKH-V0-121)", actions)
	}
	if len(ship.Conflicts) != 1 || ship.Conflicts[0].At != 1 || ship.Conflicts[0].Step != "click role=button name=Ship" || len(ship.Conflicts[0].Tests) != 2 {
		t.Fatalf("conflicts = %+v (SKH-V0-053, SKH-V0-124)", ship.Conflicts)
	}
	if len(ship.Outcomes) != 1 || ship.Outcomes[0].Matcher != "toBeVisible" || ship.OutcomeStatus != StatusResolved || len(primary.Outcomes) != 2 {
		t.Fatalf("outcomes = %+v / %+v (SKH-V0-122)", ship.Outcomes, primary.Outcomes)
	}
	if !reflect.DeepEqual(ship.Gaps, []string{"tk:note: the note text is not asserted after reload"}) {
		t.Fatalf("gaps = %v (SKH-V0-052)", ship.Gaps)
	}
	pay := byStart[id("invoices")]
	if pay.Status != CandidateBlocked || !reflect.DeepEqual(pay.BlockedBy, []string{"tk:pay"}) || pay.Support != 2 ||
		pay.Variations[0].Sources[1].Rank != "blocked" || pay.Variations[0].Sources[1].TrustLevel != "run+reviewed" {
		t.Fatalf("defect did not block its flow: %+v (SKH-V0-052, SKH-V0-054)", pay)
	}
	reports := byStart[id("reports")]
	if reports.Status != CandidateIncomplete || !reports.SingleSource || reports.OutcomeStatus != StatusUnknown {
		t.Fatalf("reports = %+v (SKH-V0-021, SKH-V0-053)", reports)
	}
	if _, ok := byStart[id("status")]; ok || len(doc.Candidates) != 3 {
		t.Fatalf("unrelated test used as a source: %+v (SKH-V0-052)", doc.Candidates)
	}
	if ship.Emit != "emitted" || pay.Emit != CandidateBlocked || reports.Emit != CandidateIncomplete {
		t.Fatalf("emit = %s %s %s (SKH-V0-125)", ship.Emit, pay.Emit, reports.Emit)
	}
	reasons := map[string]string{}
	for _, u := range doc.Unclustered {
		reasons[u.Reason] = u.Detail
	}
	if reasons["unrelated-verdict"] != "tk:status" || len(doc.Unclustered) != 2 {
		t.Fatalf("unclustered = %+v", doc.Unclustered)
	}
	if _, ok := reasons["no-start-screen"]; !ok {
		t.Fatalf("unclustered = %+v", doc.Unclustered)
	}

	request, err := InferFlowsRequest(context.Background(), m, []string{"e2e"}, trust, o)
	if err != nil {
		t.Fatal(err)
	}
	if after := skRepoState(t, root); after != before {
		t.Fatal("inference changed the worktree, index or refs (SKH-V0-005, SKH-V0-054)")
	}
	written, err := appflows.Import(root, "flows", request, appflows.FormatAdapterRequest, "")
	if err != nil || !reflect.DeepEqual(written, []string{"flows/" + ship.FlowID + ".json"}) {
		t.Fatalf("import %v %v (SKH-V0-051, SKH-V0-125)", written, err)
	}
	set, err := appflows.LoadIntents(root, "flows")
	if err != nil || len(set.Flows) != 1 {
		t.Fatalf("intents %+v %v", set, err)
	}
	intent := set.Flows[0]
	if !intent.Proposed || intent.FlowID != ship.FlowID || len(intent.Variations) != len(ship.Variations) {
		t.Fatalf("imported intent %+v (SKH-V0-051)", intent)
	}
	stepAction := map[string]string{}
	for _, s := range intent.Steps {
		stepAction[s.StepID] = s.Action
	}
	for i, v := range intent.Variations {
		got := []string{}
		for _, s := range v.Steps {
			got = append(got, stepAction[s])
		}
		want := []string{}
		for _, s := range ship.Variations[i].Steps {
			want = append(want, s.Action)
		}
		if !reflect.DeepEqual(got, want) || len(v.Outcomes) != len(ship.Variations[i].Outcomes) {
			t.Fatalf("variation %d actions %v want %v (SKH-V0-054 round trip)", i, got, want)
		}
	}
	for _, l := range intent.Links {
		if l.Basis != "inferred" || l.ReviewedAt != "" {
			t.Fatalf("imported link %+v is not inferred", l)
		}
	}
}

// SKH-V0-121 SKH-V0-123 SKH-V0-125: review regressions. A visit inside a page-object run sets the
// end screen; when two records join one test, a blocking or unrelated record wins over a
// candidate one; and an expected value that interpolates an unbound slot is never emitted.
func TestSKHV0_121_125_FlowInferenceRegressions(t *testing.T) {
	_, m := skFlowsFixture(t)
	at := Anchor{Path: "e2e/a.spec.ts", Start: 1, End: 9, Blob: strings.Repeat("1", 40)}
	shown := SKStep{Kind: StepAssert, Matcher: "toHaveText", Expected: "Welcome ${user}", Slots: []string{"user"}}
	k := &SKModel{Tests: []SKTest{{ID: "test:a", Title: "a", Path: "e2e/a.spec.ts", Complete: true, Anchor: at, Steps: []SKStep{
		{Kind: StepVisit, Target: "/orders"},
		{Kind: StepClick, Via: []string{"function:e2e/po.ts#Po.open"}},
		{Kind: StepVisit, Target: "/invoices", Via: []string{"function:e2e/po.ts#Po.open"}},
		shown,
	}}}}
	flows, _ := k.inferFlows(m, nil)
	if len(flows) != 1 || flows[0].End != screenID(m.App, "invoices") || flows[0].Emit != "no-emittable-outcome" || !flows[0].Outcomes[0].Slotted {
		t.Fatalf("flows = %+v", flows)
	}
	both := []TestTrust{
		{ID: "tk-a", Path: at.Path, Start: 1, End: 9, TrustLevel: "run+reviewed", Verdict: "bound", Selection: "candidate"},
		{ID: "tk-b", Path: at.Path, Start: 1, End: 9, TrustLevel: "run+reviewed", Verdict: "defect", Selection: "blocks"},
	}
	if flows, _ = k.inferFlows(m, both); flows[0].Status != CandidateBlocked || !reflect.DeepEqual(flows[0].BlockedBy, []string{"tk-b"}) ||
		flows[0].Variations[0].Sources[0].Rank != "blocked" {
		t.Fatalf("blocking record lost to a candidate one: %+v", flows[0])
	}
	both[1].Verdict, both[1].Selection = "unrelated", "excluded"
	if flows, rest := k.inferFlows(m, both); len(flows) != 0 || len(rest) != 1 || rest[0].Detail != "tk-b" {
		t.Fatalf("unrelated record lost to a candidate one: %+v %+v", flows, rest)
	}
}
