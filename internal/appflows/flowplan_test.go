// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// flowPlanRepo is the APK fixture (V1-5251, V1-5252): a Go module with a
// unit package `lib`, an `integration` package and an undeclared `other`
// package; a Playwright suite of three spec files, two linked by flows and
// `e2e/orphan.spec.ts` linked by none; `checkout` with two variations
// attaching the same test and `search` with one; `.corvint/flows.json`
// declaring the flows directory, the kinds rules and, when playwright is
// true, the Playwright config and its discovery receipt. Every declared link
// is reviewed at the commit that declared it.
type flowPlanFixture struct {
	root   string
	anchor string
	head   string
}

const flowPlanDeclaration = `{"profile":"corvint-flows/0","flows":"flows","kinds":[{"path":"lib","kind":"unit"},{"path":"integration","kind":"integration"}]}`
const flowPlanDeclarationPlaywright = `{"profile":"corvint-flows/0","flows":"flows","kinds":[{"path":"lib","kind":"unit"},{"path":"integration","kind":"integration"}],"playwright":{"config":"playwright.config.ts","discovery":"discovery.json"}}`

func flowPlanRepo(t *testing.T, playwright bool) flowPlanFixture {
	t.Helper()
	root := t.TempDir()
	declaration := flowPlanDeclaration
	if playwright {
		declaration = flowPlanDeclarationPlaywright
	}
	files := map[string]string{
		".gitignore":                "discovery.json\n",
		"go.mod":                    "module example.test/m\n",
		"lib/lib.go":                "package lib\n\nfunc Help() string { return \"help\" }\n",
		"lib/lib_test.go":           "package lib\n\nimport \"testing\"\n\nfunc TestHelp(t *testing.T) { Help() }\n",
		"integration/integ_test.go": "package integration\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestIntegration(t *testing.T) { lib.Help() }\n",
		"other/other.go":            "package other\n\nfunc Other() string { return \"other\" }\n",
		"other/other_test.go":       "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) { Other() }\n",
		"package.json":              `{"name":"shop","private":true}`,
		"package-lock.json":         `{"name":"shop","lockfileVersion":3}`,
		"playwright.config.ts":      `export default { testDir: "e2e", projects: [{ name: "chromium" }] };`,
		"e2e/pages/shop.ts":         `export const origin = "http://localhost:3000";`,
		"e2e/checkout.spec.ts":      "import { origin } from \"./pages/shop\";\n\ntest(\"pays\", () => origin);\n",
		"e2e/search.spec.ts":        "import { origin } from \"./pages/shop\";\n\ntest(\"finds\", () => origin);\n",
		"e2e/orphan.spec.ts":        "import { origin } from \"./pages/shop\";\n\ntest(\"wanders\", () => origin);\n",
		"src/cart.ts":               "export function total(n: number): number {\n  return n;\n}\n",
		"src/checkout.ts":           "import { total } from \"./cart\";\n\nexport function pay(n: number): number {\n  return total(n);\n}\n",
		"src/search.ts":             "export function find(q: string): string {\n  return q;\n}\n",
		FlowsDeclarationPath:        declaration,
	}
	for name, body := range files {
		writeRaw(t, root, name, []byte(body))
	}
	checkout := FlowIntent{Schema: FlowIntentSchema, FlowID: "checkout", Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
		Steps:    []FlowStep{{StepID: "pay", Action: "do pay"}},
		Outcomes: []FlowOutcome{{OutcomeID: "paid", Behavior: "paid visible", Matcher: "toBeVisible", Locator: "#paid", Value: "paid"}},
		Variations: []FlowVariation{
			{VariationID: "checkout.happy", Preconditions: []string{}, Steps: []string{"pay"}, ObservableFacts: []string{}, Outcomes: []string{"paid"}, Projects: []string{"chromium"}},
			{VariationID: "checkout.guest", Preconditions: []string{}, Steps: []string{"pay"}, ObservableFacts: []string{}, Outcomes: []string{"paid"}, Projects: []string{"chromium"}},
		},
		Links: []FlowLink{
			{From: "pay", Basis: "declared", Target: LinkTarget{Type: "source", Path: "src/checkout.ts"}},
			{From: "checkout.happy", Basis: "declared", Target: LinkTarget{Type: "test", Path: "e2e/checkout.spec.ts", TestKey: "checkout.spec.ts > pays"}},
			{From: "checkout.guest", Basis: "declared", Target: LinkTarget{Type: "test", Path: "e2e/checkout.spec.ts", TestKey: "checkout.spec.ts > pays"}},
		}}
	search := FlowIntent{Schema: FlowIntentSchema, FlowID: "search", Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
		Steps:      []FlowStep{{StepID: "query", Action: "do query"}},
		Outcomes:   []FlowOutcome{{OutcomeID: "listed", Behavior: "listed visible", Matcher: "toBeVisible", Locator: "#listed", Value: "listed"}},
		Variations: []FlowVariation{{VariationID: "search.basic", Preconditions: []string{}, Steps: []string{"query"}, ObservableFacts: []string{}, Outcomes: []string{"listed"}, Projects: []string{"chromium"}}},
		Links: []FlowLink{
			{From: "query", Basis: "declared", Target: LinkTarget{Type: "source", Path: "src/search.ts"}},
			{From: "search.basic", Basis: "declared", Target: LinkTarget{Type: "test", Path: "e2e/search.spec.ts", TestKey: "search.spec.ts > finds"}},
		}}
	writeIntent(t, root, "flows/checkout.json", checkout)
	writeIntent(t, root, "flows/search.json", search)
	gitTest(t, root, "init", "-q")
	anchor := commitAll(t, root, "intents")
	for i := range checkout.Links {
		checkout.Links[i].ReviewedAt = anchor
	}
	for i := range search.Links {
		search.Links[i].ReviewedAt = anchor
	}
	writeIntent(t, root, "flows/checkout.json", checkout)
	writeIntent(t, root, "flows/search.json", search)
	head := commitAll(t, root, "review anchors")
	f := flowPlanFixture{root: root, anchor: anchor, head: head}
	if playwright {
		f.writeDiscovery(t, head)
	}
	return f
}

// writeDiscovery writes the playwright-discovery/0 receipt a `playwright test
// --list` run would yield, bound to revision, the config bytes and the
// source digest of the tree as it is now.
func (f flowPlanFixture) writeDiscovery(t *testing.T, revision string) {
	t.Helper()
	config, err := os.ReadFile(filepath.Join(f.root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typescript.ObservePlaywrightSources(f.root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	specs, err := filepath.Glob(filepath.Join(f.root, "e2e", "*.spec.ts"))
	if err != nil {
		t.Fatal(err)
	}
	units := []typescript.PlaywrightDiscoveryUnit{}
	for _, spec := range specs {
		units = append(units, typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/" + filepath.Base(spec)})
	}
	sum := sha256.Sum256(config)
	receipt := typescript.PlaywrightDiscovery{Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])},
		Profile: "playwright-discovery/0", Revision: revision, SourceDigest: digest, Units: units}
	raw, err := jsonv2.Marshal(receipt, jsonv2.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, f.root, "discovery.json", raw)
}

// plan selects the fixture plan for dirty and joins the flows over it; it
// returns the plan before and after the join.
func (f flowPlanFixture) plan(t *testing.T, dirty ...string) (affected.Plan, affected.Plan) {
	t.Helper()
	graph, err := affected.Build(context.Background(), f.root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	before := affected.Select(graph, dirty)
	return before, JoinFlowPlan(context.Background(), f.root, graph, before, "")
}

// JoinFlowPlanForTest joins the fixture plan for dirty with the links
// evaluated at measured, the commit before the change.
func JoinFlowPlanForTest(t *testing.T, f flowPlanFixture, measured string, dirty ...string) (affected.Plan, affected.Plan) {
	t.Helper()
	graph, err := affected.Build(context.Background(), f.root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	before := affected.Select(graph, dirty)
	return before, JoinFlowPlan(context.Background(), f.root, graph, before, measured)
}

func readFile(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func e2eEntry(plan affected.Plan, path string) (affected.E2ESelection, bool) {
	if plan.E2E == nil {
		return affected.E2ESelection{}, false
	}
	for _, s := range plan.E2E.Selected {
		if s.Path == path {
			return s, true
		}
	}
	return affected.E2ESelection{}, false
}

func e2ePaths(plan affected.Plan) []string {
	var out []string
	if plan.E2E != nil {
		for _, s := range plan.E2E.Selected {
			out = append(out, s.Path)
		}
	}
	return out
}

func e2eUnknownReasons(plan affected.Plan) []string {
	var out []string
	if plan.E2E != nil {
		for _, u := range plan.E2E.Unknown {
			out = append(out, u.Reason)
		}
	}
	return out
}

func selectionKind(plan affected.Plan, unit string) (string, bool) {
	for _, s := range plan.Selected {
		if s.UnitID == unit {
			return s.TestKind, true
		}
	}
	return "", false
}

// APK-V0-001 (V1-5251): with the declaration, one plan carries the unit, the
// integration and the E2E selections, no option given; without it the plan
// is exactly the plan before the join.
func TestAPKV0001OnePlanForEveryKind_V15251(t *testing.T) {
	f := flowPlanRepo(t, true)
	before, plan := f.plan(t, "lib/lib.go", "src/checkout.ts")
	if kind, ok := selectionKind(plan, "go:example.test/m/lib"); !ok || kind != affected.TestKindUnit {
		t.Fatalf("lib: ok=%v kind=%q selected=%+v", ok, kind, plan.Selected)
	}
	if kind, ok := selectionKind(plan, "go:example.test/m/integration"); !ok || kind != affected.TestKindIntegration {
		t.Fatalf("integration: ok=%v kind=%q", ok, kind)
	}
	if plan.E2E == nil || plan.E2E.Flows != "flows" || plan.E2E.Scope != affected.E2EScopeBounded {
		t.Fatalf("e2e member: %+v", plan.E2E)
	}
	entry, ok := e2eEntry(plan, "e2e/checkout.spec.ts")
	if !ok || entry.TestKind != affected.TestKindE2E || entry.Project != "chromium" || !slices.Contains(entry.Reasons, affected.E2EReasonFlowReach) {
		t.Fatalf("checkout.spec.ts: ok=%v %+v", ok, entry)
	}
	if _, ok := e2eEntry(plan, "e2e/search.spec.ts"); ok {
		t.Fatalf("search.spec.ts is selected although no changed path reaches its flow: %+v", plan.E2E.Selected)
	}
	if reflect.DeepEqual(before, plan) {
		t.Fatal("the join changed nothing")
	}
	// Scenario: no declaration. The plan is the plan before the join.
	if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(FlowsDeclarationPath))); err != nil {
		t.Fatal(err)
	}
	before, plan = f.plan(t, "lib/lib.go", "src/checkout.ts")
	if !reflect.DeepEqual(before, plan) || plan.E2E != nil {
		t.Fatalf("without the declaration the plan changed:\n%+v\n%+v", before, plan)
	}
	for _, s := range plan.Selected {
		if s.TestKind != "" {
			t.Fatalf("testKind without a declaration: %+v", s)
		}
	}
}

// APK-V0-002 (V1-5251): the kind comes from the declared rules only; a
// selection no rule covers is UNKNOWN and stays selected.
func TestAPKV0002EverySelectionCarriesItsKind_V15251(t *testing.T) {
	f := flowPlanRepo(t, false)
	_, plan := f.plan(t, "lib/lib.go", "other/other.go")
	want := map[string]string{"go:example.test/m/lib": affected.TestKindUnit, "go:example.test/m/integration": affected.TestKindIntegration, "go:example.test/m/other": affected.TestKindUnknown}
	for unit, kind := range want {
		if got, ok := selectionKind(plan, unit); !ok || got != kind {
			t.Fatalf("%s: ok=%v kind=%q want %q (selected %+v)", unit, ok, got, kind, plan.Selected)
		}
	}
	for _, s := range plan.Selected {
		if s.TestKind == "" {
			t.Fatalf("selection without a kind: %+v", s)
		}
	}
	declared := &FlowsDeclaration{Kinds: []KindRule{{Path: ".", Kind: affected.TestKindUnit}, {Path: "test/integration", Kind: affected.TestKindIntegration}}}
	cases := map[string][]string{
		affected.TestKindUnit:        {"lib/lib_test.go"},
		affected.TestKindIntegration: {"test/integration/a_test.go", "test/integration/sub/b_test.go"},
		affected.TestKindUnknown:     {"lib/lib_test.go", "test/integration/a_test.go"},
	}
	for kind, paths := range cases {
		if got := declared.KindOf(paths); got != kind {
			t.Fatalf("KindOf(%v) = %q, want %q", paths, got, kind)
		}
	}
	if got := (&FlowsDeclaration{}).KindOf([]string{"lib/lib_test.go"}); got != affected.TestKindUnknown {
		t.Fatalf("no rules: %q", got)
	}
	if got := declared.KindOf(nil); got != affected.TestKindUnknown {
		t.Fatalf("no paths: %q", got)
	}
}

// APK-V0-003 (V1-5251): a test two variations attach appears once, naming
// both variations with the changed path of each, and its test keys.
func TestAPKV0003E2ESelectionsNameFlowAndVariation_V15251(t *testing.T) {
	f := flowPlanRepo(t, true)
	_, plan := f.plan(t, "src/cart.ts")
	entry, ok := e2eEntry(plan, "e2e/checkout.spec.ts")
	if !ok {
		t.Fatalf("checkout.spec.ts not selected: %+v", plan.E2E)
	}
	want := []affected.E2EFlowHit{
		{Flow: "checkout", Variation: "checkout.guest", ChangedPath: "src/cart.ts", Target: "src/checkout.ts"},
		{Flow: "checkout", Variation: "checkout.happy", ChangedPath: "src/cart.ts", Target: "src/checkout.ts"},
	}
	if !reflect.DeepEqual(entry.Flows, want) || !reflect.DeepEqual(entry.TestKeys, []string{"checkout.spec.ts > pays"}) || !reflect.DeepEqual(entry.Reasons, []string{affected.E2EReasonFlowReach}) {
		t.Fatalf("entry: %+v", entry)
	}
	if n := slices.Index(e2ePaths(plan), "e2e/checkout.spec.ts"); n < 0 || slices.Index(e2ePaths(plan)[n+1:], "e2e/checkout.spec.ts") >= 0 {
		t.Fatalf("checkout.spec.ts must appear once: %v", e2ePaths(plan))
	}
}

// APK-V0-004 (V1-5251): stale links, an invalid declaration and a global
// path widen the answer or mark it UNKNOWN; no test is dropped.
func TestAPKV0004FlowEvidenceFailsClosed_V15251(t *testing.T) {
	f := flowPlanRepo(t, true)
	// Stale: the search link's target changed after its review.
	writeRaw(t, f.root, "src/search.ts", []byte("export function find(q: string): string {\n  return q + q;\n}\n"))
	head := commitAll(t, f.root, "search changed after review")
	f.writeDiscovery(t, head)
	_, plan := f.plan(t, "lib/lib.go")
	entry, ok := e2eEntry(plan, "e2e/search.spec.ts")
	if !ok || !slices.Contains(entry.Reasons, affected.E2EReasonStaleLink) || len(entry.Flows) != 0 {
		t.Fatalf("stale flow's test not selected: ok=%v %+v", ok, entry)
	}
	if _, ok := e2eEntry(plan, "e2e/checkout.spec.ts"); ok {
		t.Fatalf("checkout.spec.ts selected although its links are reviewed and unreached: %+v", plan.E2E.Selected)
	}
	if plan.E2E.Scope != affected.E2EScopeUnknown || !reflect.DeepEqual(e2eUnknownReasons(plan), []string{affected.UnknownE2EFlowStale}) || plan.E2E.Unknown[0].Detail != "search" {
		t.Fatalf("stale evidence not named: scope=%s unknown=%+v", plan.E2E.Scope, plan.E2E.Unknown)
	}
	// The plan's own members are untouched by the E2E member.
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("plan scope changed: %s %+v", plan.Scope, plan.Unknown)
	}
	// Measured at the base: a change to the reviewed target itself is a
	// reach, not a stale link (the e2e-safe model, AFU-V1-020).
	_, plan = JoinFlowPlanForTest(t, f, f.head, "src/search.ts")
	if entry, ok := e2eEntry(plan, "e2e/search.spec.ts"); !ok || !reflect.DeepEqual(entry.Reasons, []string{affected.E2EReasonFlowReach}) || plan.E2E.Scope != affected.E2EScopeBounded {
		t.Fatalf("change to a reviewed target measured at its base: ok=%v %+v scope=%s unknown=%+v", ok, entry, plan.E2E.Scope, plan.E2E.Unknown)
	}
	// Unreviewed: an inferred link has no narrowing authority, so the
	// flow's tests run and the plan says why.
	var searchIntent FlowIntent
	if err := json.Unmarshal([]byte(readFile(t, f.root, "flows/search.json")), &searchIntent); err != nil {
		t.Fatal(err)
	}
	for i := range searchIntent.Links {
		searchIntent.Links[i].Basis, searchIntent.Links[i].ReviewedAt = "inferred", ""
	}
	writeIntent(t, f.root, "flows/search.json", searchIntent)
	head = commitAll(t, f.root, "search links inferred")
	f.writeDiscovery(t, head)
	_, plan = f.plan(t, "lib/lib.go")
	if entry, ok := e2eEntry(plan, "e2e/search.spec.ts"); !ok || !reflect.DeepEqual(entry.Reasons, []string{affected.E2EReasonUnreviewedLink}) || !reflect.DeepEqual(e2eUnknownReasons(plan), []string{affected.UnknownE2EFlowUnreviewed}) || plan.E2E.Scope != affected.E2EScopeUnknown {
		t.Fatalf("unreviewed flow: ok=%v %+v unknown=%+v", ok, entry, plan.E2E.Unknown)
	}
	// Global: a lockfile change selects every inventory file.
	_, plan = f.plan(t, "package-lock.json")
	if plan.E2E.Scope != affected.E2EScopeFull || !slices.Contains(e2eUnknownReasons(plan), affected.UnknownE2EGlobalPath) {
		t.Fatalf("global path: scope=%s unknown=%+v", plan.E2E.Scope, plan.E2E.Unknown)
	}
	if got := e2ePaths(plan); !reflect.DeepEqual(got, []string{"e2e/checkout.spec.ts", "e2e/orphan.spec.ts", "e2e/search.spec.ts"}) {
		t.Fatalf("global path selected %v", got)
	}
	for _, s := range plan.E2E.Selected {
		if !slices.Contains(s.Reasons, affected.E2EReasonGlobalPathChanged) {
			t.Fatalf("%s lacks the global reason: %+v", s.Path, s)
		}
	}
	// Invalid declaration: UNKNOWN, nothing narrowed, the unknown names the file.
	for _, body := range []string{`{"profile":"corvint-flows/0"}`, `{"profile":"corvint-flows/0","flows":"../out"}`, `{"profile":"corvint-flows/0","flows":"missing"}`, `{"profile":"corvint-flows/0","flows":"flows","kinds":[{"path":"lib","kind":"e2e"}]}`, `{"profile":"corvint-flows/0","flows":"flows","extra":1}`} {
		writeRaw(t, f.root, FlowsDeclarationPath, []byte(body))
		_, plan = f.plan(t, "lib/lib.go")
		if plan.E2E == nil || plan.E2E.Scope != affected.E2EScopeUnknown || len(plan.E2E.Selected) != 0 || !reflect.DeepEqual(e2eUnknownReasons(plan), []string{affected.UnknownFlowsDeclarationInvalid}) || plan.E2E.Unknown[0].Detail != FlowsDeclarationPath {
			t.Fatalf("%s: %+v", body, plan.E2E)
		}
	}
}

// APK-V0-006 (V1-5251): two joins over the same inputs encode to the same
// bytes and leave the worktree and Git state as they were.
func TestAPKV0006DeterministicAndReadOnly_V15251(t *testing.T) {
	f := flowPlanRepo(t, true)
	status := gitOut(t, f.root, "status", "--porcelain", "--untracked-files=all")
	_, first := f.plan(t, "src/checkout.ts", "lib/lib.go")
	_, second := f.plan(t, "src/checkout.ts", "lib/lib.go")
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("plans differ:\n%s\n%s", a, b)
	}
	if after := gitOut(t, f.root, "status", "--porcelain", "--untracked-files=all"); after != status {
		t.Fatalf("the join wrote to the repository:\n%s\nwas:\n%s", after, status)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".corvint", "flows.json")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(f.root, ".corvint"))
	if err != nil || len(entries) != 1 {
		t.Fatalf(".corvint gained entries: %v %v", entries, err)
	}
}

// APK-V0-007 (V1-5251): every test key and variation the flows impact report
// names for the same inputs is in the E2E selection.
func TestAPKV0007AgreesWithFlowsImpact_V15251(t *testing.T) {
	f := flowPlanRepo(t, true)
	ctx := context.Background()
	graph, err := affected.Build(ctx, f.root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"src/cart.ts", "src/search.ts"}
	plan := affected.Select(graph, dirty)
	joined := JoinFlowPlan(ctx, f.root, graph, plan, "")
	set, err := LoadIntentsAt(ctx, f.root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := FlowImpact(ctx, f.root, set, f.anchor, graph, plan)
	if err != nil {
		t.Fatal(err)
	}
	var report ImpactReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Flows) != 2 {
		t.Fatalf("impact reached %d flows: %s", len(report.Flows), raw)
	}
	for _, flow := range report.Flows {
		for _, key := range flow.TestKeys {
			if !slices.ContainsFunc(joined.E2E.Selected, func(s affected.E2ESelection) bool { return slices.Contains(s.TestKeys, key) }) {
				t.Fatalf("impact test key %q is not an E2E selection: %+v", key, joined.E2E.Selected)
			}
		}
		for _, variation := range flow.Variations {
			if !slices.ContainsFunc(joined.E2E.Selected, func(s affected.E2ESelection) bool {
				return slices.ContainsFunc(s.Flows, func(h affected.E2EFlowHit) bool { return h.Flow == flow.FlowID && h.Variation == variation })
			}) {
				t.Fatalf("impact variation %s/%s is not named: %+v", flow.FlowID, variation, joined.E2E.Selected)
			}
		}
	}
}

// AFU-V1-140, AFU-V1-141 (V1-5252): the inventory is the bound receipt's
// units, no provider file read; the test no flow links is selected for an
// unrelated change and reported as unlinked.
func TestAFUV1140DerivedInventorySelectsUnlinkedTest_V15252(t *testing.T) {
	f := flowPlanRepo(t, true)
	_, plan := f.plan(t, "lib/lib.go")
	if plan.E2E.Inventory.Source != affected.E2EInventoryDiscovery || plan.E2E.Inventory.State != "MATCHED" || plan.E2E.Inventory.Count != 3 {
		t.Fatalf("inventory: %+v", plan.E2E.Inventory)
	}
	if got := e2ePaths(plan); !reflect.DeepEqual(got, []string{"e2e/orphan.spec.ts"}) {
		t.Fatalf("selected %v, want only the unlinked test", got)
	}
	entry, _ := e2eEntry(plan, "e2e/orphan.spec.ts")
	if !reflect.DeepEqual(entry.Reasons, []string{affected.E2EReasonUnlinked}) || entry.Project != "chromium" || !reflect.DeepEqual(plan.E2E.Inventory.Unlinked, []string{"e2e/orphan.spec.ts"}) {
		t.Fatalf("unlinked: %+v inventory %+v", entry, plan.E2E.Inventory)
	}
	if plan.E2E.Scope != affected.E2EScopeBounded {
		t.Fatalf("an unlinked test is a bounded answer: %+v", plan.E2E)
	}
	// A new test is in the inventory once the receipt is refreshed.
	writeRaw(t, f.root, "e2e/returns.spec.ts", []byte("import { origin } from \"./pages/shop\";\n\ntest(\"returns\", () => origin);\n"))
	head := commitAll(t, f.root, "add a test")
	f.writeDiscovery(t, head)
	_, plan = f.plan(t, "lib/lib.go")
	if plan.E2E.Inventory.Count != 4 || !slices.Contains(e2ePaths(plan), "e2e/returns.spec.ts") {
		t.Fatalf("new test not inventoried: %+v %v", plan.E2E.Inventory, e2ePaths(plan))
	}
	// Without the playwright member the inventory is the flows' test links.
	writeRaw(t, f.root, FlowsDeclarationPath, []byte(flowPlanDeclaration))
	_, plan = f.plan(t, "lib/lib.go")
	if plan.E2E.Inventory.Source != affected.E2EInventoryFlows || plan.E2E.Inventory.Count != 2 || len(plan.E2E.Selected) != 0 || plan.E2E.Scope != affected.E2EScopeBounded {
		t.Fatalf("flows inventory: %+v", plan.E2E)
	}
}

// AFU-V1-142, AFU-V1-143 (V1-5252): a missing, stale or malformed receipt and
// an untracked config each give a FULL answer with the typed unknown, never an
// empty bounded one.
func TestAFUV1142UnderivableInventoryFailsClosed_V15252(t *testing.T) {
	f := flowPlanRepo(t, true)
	check := func(label, detail string) {
		t.Helper()
		_, plan := f.plan(t, "lib/lib.go")
		if plan.E2E.Scope != affected.E2EScopeFull || !reflect.DeepEqual(e2eUnknownReasons(plan), []string{affected.UnknownE2EInventoryUnderivable}) || plan.E2E.Unknown[0].Detail != detail {
			t.Fatalf("%s: scope=%s unknown=%+v", label, plan.E2E.Scope, plan.E2E.Unknown)
		}
		if got := e2ePaths(plan); !reflect.DeepEqual(got, []string{"e2e/checkout.spec.ts", "e2e/search.spec.ts"}) {
			t.Fatalf("%s: selected %v", label, got)
		}
		for _, s := range plan.E2E.Selected {
			if !slices.Contains(s.Reasons, affected.E2EReasonInventoryUnderivable) {
				t.Fatalf("%s: %+v", label, s)
			}
		}
		if !strings.Contains(plan.E2E.Unknown[0].Hint, "corvint affected discovery") {
			t.Fatalf("%s: hint names no repair: %q", label, plan.E2E.Unknown[0].Hint)
		}
	}
	if err := os.Remove(filepath.Join(f.root, "discovery.json")); err != nil {
		t.Fatal(err)
	}
	check("missing", "discovery:MISSING")
	writeRaw(t, f.root, "discovery.json", []byte("{not json"))
	check("malformed", "discovery:MALFORMED")
	// Stale: the receipt was taken before a test file changed.
	f.writeDiscovery(t, f.head)
	writeRaw(t, f.root, "e2e/orphan.spec.ts", []byte("test(\"wanders further\", () => 1);\n"))
	commitAll(t, f.root, "test changed after discovery")
	check("stale", "discovery:BINDING_MISMATCH")
	// Untracked config.
	writeRaw(t, f.root, FlowsDeclarationPath, []byte(strings.Replace(flowPlanDeclarationPlaywright, "playwright.config.ts", "pw.local.config.ts", 1)))
	writeRaw(t, f.root, "pw.local.config.ts", []byte("export default {};"))
	check("untracked config", "config:untracked")
}

// AFU-V1-145 (V1-5252): with a provider, the member says the provider's
// selection wins and lists the derived files the provider omits.
func TestAFUV1145ProviderPrecedenceIsRecorded_V15252(t *testing.T) {
	f := flowPlanRepo(t, true)
	writeRaw(t, f.root, "e2e-provider.json", []byte(`{"schema":"application-flow-selection-provider/1","flows":"flows","runner_config":"playwright.config.ts","discovery":"discovery.json","global_paths":[],"inventory":[{"path":"e2e/checkout.spec.ts","project":"chromium","test_key":"checkout.spec.ts > pays"},{"path":"e2e/search.spec.ts","project":"chromium","test_key":"search.spec.ts > finds"}]}`))
	_, plan := f.plan(t, "package.json")
	ReconcileProvider(f.root, "e2e-provider.json", plan.E2E)
	if plan.E2E.Inventory.Source != affected.E2EInventoryProvider || !reflect.DeepEqual(plan.E2E.Inventory.ProviderOmits, []string{"e2e/orphan.spec.ts"}) {
		t.Fatalf("provider reconciliation: %+v", plan.E2E.Inventory)
	}
	ReconcileProvider(f.root, "", plan.E2E)
	ReconcileProvider(f.root, "missing.json", plan.E2E)
	if plan.E2E.Inventory.Source != affected.E2EInventoryProvider {
		t.Fatalf("an absent or unreadable provider changed the member: %+v", plan.E2E.Inventory)
	}
}
