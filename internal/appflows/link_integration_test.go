// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

const cartFile = "e2e/checkout.spec.ts"
const linkedCart = "playwright:" + cartFile + "::chromium::checkout"
const otherCart = "playwright:" + cartFile + "::chromium::guest"
const pyCart = "pytest:tests/test_cart.py::test_cart"

func linkFixture(t *testing.T) flowPlanFixture {
	t.Helper()
	return linkFixtureWithSource(t, "import { pay } from '../src/checkout';\n// @flow:checkout\ntest('checkout',()=>pay(1));\ntest('guest',()=>{});\n")
}

func linkFixtureWithSource(t *testing.T, source string) flowPlanFixture {
	t.Helper()
	f := flowPlanRepo(t, true)
	d, err := ReadFlowsDeclaration(f.root)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(flowsDeclarationFile{Profile: "corvint-flows/0", Flows: d.Dir, Playwright: &flowsPlaywrightMember{Config: d.PlaywrightConfig, Discovery: d.PlaywrightDiscovery}})
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, f.root, FlowsDeclarationPath, body)
	writeRaw(t, f.root, cartFile, []byte(source))
	// Only search imports this application file. Checkout can still fetch it by URL.
	const searchSpec = "e2e/search.spec.ts"
	writeRaw(t, f.root, searchSpec, []byte("import '../src/search';\n"+readFile(t, f.root, searchSpec)))
	writeRaw(t, f.root, "pytest.ini", []byte("[pytest]\n"))
	writeRaw(t, f.root, "tests/test_cart.py", []byte("def test_cart(page): pass\n"))
	var inv testinventory.Inventory
	inv.Profile = testinventory.Profile
	instance := func(runner, config string, tests []testinventory.Test) testinventory.Instance {
		ident, _ := testinventory.CurrentLister(runner)
		bound := func(path string) string { return testinventory.BlobID("sha1", []byte(readFile(t, f.root, path))) }
		in := testinventory.Instance{Key: runner + "@.", Runner: runner, Root: ".", Config: config, ConfigDigest: bound(config), Tests: tests, Commit: f.head, Lister: testinventory.Lister{Name: ident.Name, Version: ident.Version, Kind: ident.Kind}}
		if in.Lister.Kind == testinventory.KindExecuting {
			in.Lister.Command = []string{"fixture-list"}
		}
		if runner == testid.Playwright {
			in.Lockfile = "package-lock.json"
			in.LockfileDigest = bound(in.Lockfile)
		}
		files := map[string]bool{}
		for _, test := range tests {
			if !files[test.File] {
				in.Files = append(in.Files, testinventory.File{Path: test.File, Blob: bound(test.File)})
				files[test.File] = true
			}
		}
		return in
	}
	inv.Instances = []testinventory.Instance{instance(testid.Playwright, "playwright.config.ts", []testinventory.Test{
		{ID: linkedCart, File: cartFile}, {ID: otherCart, File: cartFile},
		{ID: "playwright:e2e/search.spec.ts::chromium::finds", File: "e2e/search.spec.ts"},
		{ID: "playwright:e2e/orphan.spec.ts::chromium::wanders", File: "e2e/orphan.spec.ts"}}),
		instance(testid.Pytest, "pytest.ini", []testinventory.Test{{ID: pyCart, File: "tests/test_cart.py", Kind: "e2e", KindSource: "runner"}})}
	body, err = testinventory.Encode(inv)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, f.root, testinventory.Path, body)
	f.head = commitAll(t, f.root, "canonical browser inventory")
	return f
}

func TestFLPV1001PrivateBulkProposals(t *testing.T) {
	f := linkFixture(t)
	before := gitOut(t, f.root, "status", "--porcelain")
	result, err := ProposeLinks(t.Context(), f.root, "checkout", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) != 10 || result.Commit != f.head {
		t.Fatalf("proposals %+v", result)
	}
	raw, err := os.ReadFile(filepath.Join(f.root, ".git", "corvint", proposalFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"confidence":1.00`) || gitOut(t, f.root, "status", "--porcelain") != before {
		t.Fatalf("private proposals or confidence: %s", raw)
	}
	again, err := ProposeLinks(t.Context(), f.root, "checkout", true)
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatal("non-deterministic proposals", err)
	}
	acceptCart(t, f, linkedCart)
	only, err := ProposeLinks(t.Context(), f.root, "checkout", false)
	if err != nil {
		t.Fatal(err)
	}
	bulk, err := ProposeLinks(t.Context(), f.root, "checkout", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(only.Proposals) != 8 || len(bulk.Proposals) != 10 {
		t.Fatalf("unassigned %d, bulk %d", len(only.Proposals), len(bulk.Proposals))
	}
}

func TestFLPV1003ConfidenceInputs(t *testing.T) {
	f := linkFixture(t)
	graph, err := affected.Build(t.Context(), f.root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := testid.Parse(linkedCart)
	for _, tc := range []struct{ name, flow, source, target, score string }{
		{"title", "checkout", "", "", "0.35"}, {"tag", "orders", "// @flow:orders", "", "0.25"},
		{"static", "orders", "", "src/checkout.ts", "0.40"}, {"absent", "orders", "@flow:orders-more", "", "0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := linkVariation{flow: tc.flow, id: "v"}
			if tc.target != "" {
				v.sources = []string{tc.target}
			}
			score, evidence := linkScore(id, v, []byte(tc.source), graph, f.head)
			if string(score) != tc.score || len(evidence) != 3 {
				t.Fatal(score, evidence)
			}
			for _, e := range evidence {
				if !strings.HasSuffix(e, ":absent") && !strings.Contains(e, f.head) {
					t.Fatal("unbound citation", e)
				}
			}
		})
	}
}

func acceptCart(t *testing.T, f flowPlanFixture, only ...string) {
	t.Helper()
	if _, err := ProposeLinks(t.Context(), f.root, "checkout", true); err != nil {
		t.Fatal(err)
	}
	n, err := AcceptLinks(t.Context(), f.root, "checkout", "Reviewer", "checkout.happy", only)
	if err != nil || n != len(only) {
		t.Fatalf("accept count %d, error %v", n, err)
	}
	commitAll(t, f.root, "review selected checkout links")
}

func TestFLPV1007AcceptedOnly(t *testing.T) {
	f := linkFixture(t)
	graph, err := affected.Build(t.Context(), f.root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	plan := func(dirty string) affected.Plan {
		before := affected.Select(graph, []string{dirty})
		return JoinFlowPlan(context.Background(), f.root, graph, before, "")
	}
	result := plan("e2e/search.spec.ts")
	before, ok := e2eEntry(result, cartFile)
	if !ok || !slices.Contains(before.Reasons, "unassigned") || !slices.Equal(before.TestKeys, []string{linkedCart, otherCart}) {
		t.Fatalf("tag alone: %+v", result.E2E)
	}
	if _, err := ProposeLinks(t.Context(), f.root, "", true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.E2E, plan("e2e/search.spec.ts").E2E) {
		t.Fatal("proposals altered selection")
	}
	acceptCart(t, f, linkedCart, pyCart)
	after := plan("e2e/search.spec.ts")
	selected, _ := e2eEntry(after, cartFile)
	if !slices.Equal(selected.TestKeys, []string{otherCart}) {
		t.Fatalf("mixed file: %+v; unknown %+v", after.E2E, after.Unknown)
	}
	if after.FullInstances["pytest@."] != browserGroup {
		t.Fatalf("pytest group %+v", after.FullInstances)
	}
	hit := plan("src/checkout.ts")
	reached, _ := e2eEntry(hit, cartFile)
	if !slices.Contains(reached.TestKeys, linkedCart) || len(reached.Flows) == 0 {
		t.Fatalf("flow reach %+v; unknown %+v", hit.E2E, hit.Unknown)
	}
	if !slices.ContainsFunc(hit.Selected, func(s affected.Selection) bool { return s.Tier == "flow" && slices.Contains(s.Tests, cartFile) }) {
		t.Fatalf("missing flow tier %+v", hit.Selected)
	}
}

func TestFLPV1009DoubtfulLinksRun(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		edit       func(*testing.T, flowPlanFixture)
	}{
		{"test content", linkStale, func(t *testing.T, f flowPlanFixture) { writeRaw(t, f.root, cartFile, []byte("// edited\n")) }},
		{"variation", linkStale, func(t *testing.T, f flowPlanFixture) {
			set, _ := LoadIntents(f.root, "flows")
			intent := set.Flows[0]
			intent.Variations[0].ObservableFacts = []string{"different"}
			writeIntent(t, f.root, "flows/checkout.json", intent)
		}},
		{"disagreement", linkDisagrees, func(t *testing.T, f flowPlanFixture) {}},
		{"unknown", linkUnknown, func(t *testing.T, f flowPlanFixture) {
			set, _ := LoadIntents(f.root, "flows")
			intent := set.Flows[0]
			intent.Links = slices.DeleteFunc(intent.Links, func(l FlowLink) bool { return l.Target.Type == "source" })
			writeIntent(t, f.root, "flows/checkout.json", intent)
			commitAll(t, f.root, "no source targets")
		}},
		{"dangling variation", linkDangling, func(t *testing.T, f flowPlanFixture) {
			d, _ := ReadFlowsDeclaration(f.root)
			d.Links[0].VariationID = "gone"
			b, _ := EncodeFlowsDeclaration(*d)
			writeRaw(t, f.root, FlowsDeclarationPath, b)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := linkFixture(t)
			acceptCart(t, f, linkedCart)
			tc.edit(t, f)
			_, plan := f.plan(t, cartFile)
			sel, ok := e2eEntry(plan, cartFile)
			if !ok || !slices.ContainsFunc(plan.E2E.Selected, func(s affected.E2ESelection) bool {
				return slices.Contains(s.TestKeys, linkedCart) && slices.Contains(s.Reasons, tc.want)
			}) {
				t.Fatalf("wanted %s got %+v; %v", tc.want, plan.E2E, sel)
			}
		})
	}
}

func TestFLPV1010DanglingTest(t *testing.T) {
	f := linkFixture(t)
	acceptCart(t, f, linkedCart)
	var inv testinventory.Inventory
	if err := json.Unmarshal([]byte(readFile(t, f.root, testinventory.Path)), &inv); err != nil {
		t.Fatal(err)
	}
	for i := range inv.Instances {
		for j := range inv.Instances[i].Tests {
			if inv.Instances[i].Tests[j].ID == linkedCart {
				inv.Instances[i].Tests[j].ID = linkedCart + "renamed"
			}
		}
	}
	b, err := testinventory.Encode(inv)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, f.root, testinventory.Path, b)
	commitAll(t, f.root, "renamed test")
	_, plan := f.plan(t, "src/search.ts")
	if !slices.ContainsFunc(plan.E2E.Unknown, func(u affected.Unknown) bool {
		return u.Reason == linkDangling && strings.Contains(u.Detail, linkedCart)
	}) {
		t.Fatal(plan.E2E)
	}
	sel, _ := e2eEntry(plan, cartFile)
	set, err := LoadIntentsAt(t.Context(), f.root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := FlowStatus(t.Context(), f.root, set, StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var status StatusReport
	if err = json.Unmarshal(raw, &status); err != nil || status.Unassigned == nil || *status.Unassigned != 5 || !strings.Contains(string(raw), linkDangling) {
		t.Fatalf("status %s: %v", raw, err)
	}
	// Both reports must evaluate the same immutable revision even after a local edit.
	writeRaw(t, f.root, FlowsDeclarationPath, []byte(`{"profile":"corvint-flows/0","flows":"flows"}`))
	gaps, err := FlowGaps(t.Context(), f.root, set, nil, "", nil)
	var g GapsReport
	if err != nil || json.Unmarshal(gaps, &g) != nil || g.UnlinkedTotal != 5 || !slices.ContainsFunc(g.UnlinkedTests, func(row Gap) bool { return row.TestKey == linkedCart+"renamed" }) {
		t.Fatalf("gaps %s: %v", gaps, err)
	}
	if !slices.Contains(sel.Reasons, "unassigned") || !slices.Contains(sel.TestKeys, linkedCart+"renamed") {
		t.Fatal(sel)
	}
}

func TestFLPV1009CanonicalVariationDigest(t *testing.T) {
	digest := func(text string) string {
		var set IntentSet
		var intent FlowIntent
		raw := []byte(`{"variations":[` + text + `]}`)
		if err := json.Unmarshal(raw, &intent); err != nil {
			t.Fatal(err)
		}
		if err := set.bindVariationDigests(raw, intent); err != nil {
			t.Fatal(err)
		}
		return set.variationDigests["v"]
	}
	first := digest(`{"steps":[],"variation_id":"v"}`)
	if first != digest(`{ "variation_id": "v", "steps": [] }`) || first == digest(`{"variation_id":"v","steps":null}`) || first == digest(`{"variation_id":"v"}`) {
		t.Fatal("canonical record failed to bind variation content")
	}
}

func TestFLPV1010UnresolvedRuntimeRetainsAcceptedTest(t *testing.T) {
	f := linkFixtureWithSource(t, "test('checkout', async ({page}) => { await page.goto('/checkout') });\ntest('guest',()=>{});\n")
	writeRaw(t, f.root, "server/checkout.go", []byte("package server\nimport \"net/http\"\nfunc Routes(mux *http.ServeMux) { mux.HandleFunc(\"/checkout\", Checkout) }\nfunc Checkout(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }\n"))
	commitAll(t, f.root, "serve checkout")
	acceptCart(t, f, linkedCart)
	graph, err := affected.Build(t.Context(), f.root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	control := JoinFlowPlan(t.Context(), f.root, graph, affected.Select(graph, []string{"e2e/search.spec.ts"}), "")
	row, ok := e2eEntry(control, cartFile)
	if !ok || !slices.Equal(row.TestKeys, []string{otherCart}) {
		t.Fatal("the search spec has a closed exclusion", row)
	}
	for _, tc := range []struct {
		name  string
		dirty []string
	}{
		{"browser route", []string{"server/checkout.go"}},
		{"mixed paths", []string{"e2e/search.spec.ts", "server/checkout.go"}},
		{"future frontier", []string{"e2e/search.spec.ts"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := affected.Select(graph, tc.dirty)
			if tc.name == "future frontier" {
				plan.Unknown = []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: "future:runtime-binding:e2e/search.spec.ts"}}
				plan.Scope = affected.ScopeUnknown
			}
			got := JoinFlowPlan(t.Context(), f.root, graph, plan, "")
			sel, ok := e2eEntry(got, cartFile)
			if !ok || !slices.Contains(sel.TestKeys, linkedCart) || !slices.Contains(sel.Reasons, linkUnknown) || !slices.Contains(sel.Reasons, "unassigned") {
				t.Fatalf("open frontier %+v dropped checkout: %+v", plan.Unknown, sel)
			}
		})
	}
}

func TestFLPV1007DeclaredDiscoverySurvivesOtherInventories(t *testing.T) {
	for _, name := range []string{"go only", "other Playwright config", "missing receipt"} {
		t.Run(name, func(t *testing.T) {
			f := linkFixture(t)
			const config = "e2e.config.ts"
			writeRaw(t, f.root, config, []byte(readFile(t, f.root, "playwright.config.ts")))
			writeRaw(t, f.root, FlowsDeclarationPath, []byte(strings.Replace(flowPlanDeclarationPlaywright, "playwright.config.ts", config, 1)))
			if name != "other Playwright config" {
				for _, file := range []string{"playwright.config.ts", "pytest.ini", "tests/test_cart.py"} {
					if err := os.Remove(filepath.Join(f.root, file)); err != nil {
						t.Fatal(err)
					}
				}
				lister, _ := testinventory.CurrentLister(testid.Go)
				inv := testinventory.Inventory{Profile: testinventory.Profile, Instances: []testinventory.Instance{{
					Key: "go@.", Root: ".", Runner: testid.Go, Config: "go.mod", ConfigDigest: testinventory.BlobID("sha1", []byte(readFile(t, f.root, "go.mod"))),
					Commit: f.head, Lister: testinventory.Lister{Name: lister.Name, Version: lister.Version, Kind: lister.Kind},
				}}}
				body, err := testinventory.Encode(inv)
				if err != nil {
					t.Fatal(err)
				}
				writeRaw(t, f.root, testinventory.Path, body)
			}
			f.head = commitAll(t, f.root, "declare custom browser config")
			f.writeDiscovery(t, f.head)
			if name == "missing receipt" {
				if err := os.Remove(filepath.Join(f.root, "discovery.json")); err != nil {
					t.Fatal(err)
				}
			}
			_, got := f.plan(t, "lib/lib.go")
			if got.E2E.Inventory.Source != affected.E2EInventoryDiscovery {
				t.Fatal(got.E2E)
			}
			if name == "missing receipt" {
				if got.E2E.Scope != affected.E2EScopeFull || !slices.Contains(e2eUnknownReasons(got), affected.UnknownE2EInventoryUnderivable) {
					t.Fatal(got.E2E)
				}
				return
			}
			if got.E2E.Inventory.State != "MATCHED" {
				t.Fatal(got.E2E)
			}
			for _, path := range []string{cartFile, "e2e/search.spec.ts", "e2e/orphan.spec.ts"} {
				sel, ok := e2eEntry(got, path)
				if !ok || sel.Project != "chromium" || len(sel.TestKeys) != 0 {
					t.Fatalf("declared suite lost whole file %s: %+v", path, got.E2E)
				}
				if name == "other Playwright config" && (!slices.Contains(sel.Configs, config) ||
					!slices.Contains(sel.Configs, "playwright.config.ts")) {
					t.Fatalf("shared project and file lost a config: %+v", sel)
				}
			}
			if name == "other Playwright config" && got.FullInstances["pytest@."] == "" {
				t.Fatal("lost the independent pytest group", got.FullInstances)
			}
		})
	}
}

func TestFLPV1001MixedLinksAndVariationGaps(t *testing.T) {
	f := linkFixture(t)
	set, err := LoadIntents(f.root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	intent := set.Flows[0]
	intent.Links = slices.DeleteFunc(intent.Links, func(l FlowLink) bool { return l.Target.Type == "test" })
	intent.Links = append(intent.Links, FlowLink{From: "pay", Basis: "declared", Target: LinkTarget{Type: "source", Path: "src/cart.ts"}})
	writeIntent(t, f.root, "flows/checkout.json", intent)
	commitAll(t, f.root, "only canonical test links")
	acceptCart(t, f, linkedCart)
	if _, err := ProposeLinks(t.Context(), f.root, "checkout", true); err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptLinks(t.Context(), f.root, "checkout", "Reviewer", "checkout.guest", []string{linkedCart}); err != nil {
		t.Fatal(err)
	}
	commitAll(t, f.root, "second accepted variation")
	for _, paths := range [][]string{{"src/cart.ts"}, {"src/cart.ts", "src/checkout.ts"}} {
		_, both := f.plan(t, paths...)
		selected, _ := e2eEntry(both, cartFile)
		var want []affected.E2EFlowHit
		for _, variation := range []string{"checkout.guest", "checkout.happy"} {
			for _, changed := range paths {
				want = append(want, affected.E2EFlowHit{Flow: "checkout", Variation: variation, ChangedPath: changed, Target: changed})
			}
		}
		if !slices.Equal(selected.Flows, want) || !slices.Contains(selected.Reasons, affected.E2EReasonFlowReach) {
			t.Fatalf("selected hits %+v, want %+v", selected, want)
		}
	}
	intent.Variations[1].ObservableFacts = []string{"a different result"}
	writeIntent(t, f.root, "flows/checkout.json", intent)
	commitAll(t, f.root, "stale guest variation")

	proposals, err := ProposeLinks(t.Context(), f.root, "", false)
	if err != nil || !slices.ContainsFunc(proposals.Proposals, func(p LinkProposal) bool { return p.Test == linkedCart }) {
		t.Fatalf("mixed links dropped from default proposals: %+v %v", proposals, err)
	}
	_, plan := f.plan(t, "src/search.ts")
	row, _ := e2eEntry(plan, cartFile)
	if !slices.Contains(row.TestKeys, linkedCart) || !slices.Contains(row.Reasons, linkStale) {
		t.Fatal(row)
	}
	set, err = LoadIntentsAt(t.Context(), f.root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := FlowGaps(t.Context(), f.root, set, nil, "", nil)
	var report GapsReport
	if err != nil || json.Unmarshal(raw, &report) != nil {
		t.Fatalf("gaps %s %v", raw, err)
	}
	has := func(code, member string) bool {
		return slices.ContainsFunc(report.Flows[0].Gaps, func(g Gap) bool { return g.Code == code && g.Member == member })
	}
	if has(GapNoTest, "checkout.happy") || !has(GapTestWithoutAssertion, "checkout.happy") || !has(GapNoTest, "checkout.guest") {
		t.Fatal(report.Flows)
	}
}

func TestFLPV1010BrowserLoadedApplicationFile(t *testing.T) {
	const asset = "src/search.ts"
	f := linkFixtureWithSource(t, `test('checkout', async ({page}) => {
  const url = '/' + ['src', 'search'].join('/') + '.ts';
  const response = await page.goto(url);
  expect(await response.text()).toContain('return q');
});
test('guest', () => {});
`)
	acceptCart(t, f, linkedCart, otherCart)
	writeRaw(t, f.root, asset, []byte("export function find(q: string) { return ''; }\n"))
	closures, err := typescript.PlaywrightTestClosures(f.root, "playwright.config.ts")
	if err != nil || !slices.ContainsFunc(closures, func(c typescript.PlaywrightTestClosure) bool {
		return c.Test == cartFile && slices.Contains(c.Excludable, asset)
	}) {
		t.Fatalf("fixture must admit the static exclusion: %+v, %v", closures, err)
	}
	_, plan := f.plan(t, asset)
	row, ok := e2eEntry(plan, cartFile)
	if !ok || !slices.Equal(row.TestKeys, []string{linkedCart, otherCart}) || !slices.Contains(row.Reasons, linkUnknown) {
		t.Fatalf("URL-loaded asset lost accepted checkout tests: %+v", row)
	}
}

func TestFLPV1010ProjectTestDirBounds(t *testing.T) {
	f := linkFixture(t)
	const config = "playwright.config.ts"
	const otherSpec = "e2e/search.spec.ts"
	writeRaw(t, f.root, config, []byte(`export default {
		testDir: "src", projects: [{ name: "chromium", testDir: "e2e" }]
	};`))
	key := e2eFile{"chromium", cartFile}
	if !provenRuntimeExclusions(f.root, config, []string{otherSpec})[key] ||
		provenRuntimeExclusions(f.root, config, []string{otherSpec, "src/search.ts"})[key] {
		t.Fatal("project testDir must override the config directory for every dirty file")
	}
	const nested = "e2e/implicit.config.ts"
	writeRaw(t, f.root, nested, []byte(`export default { projects: [{ name: "chromium" }] };`))
	if !provenRuntimeExclusions(f.root, nested, []string{otherSpec})[key] ||
		provenRuntimeExclusions(f.root, nested, []string{"src/search.ts"})[key] {
		t.Fatal("implicit testDir must be the nested config's directory")
	}
	writeRaw(t, f.root, config, []byte(`export default {
		testDir: "e2e", projects: [{ name: "chromium", testDir: process.env.TEST_DIR }]
	};`))
	if provenRuntimeExclusions(f.root, config, []string{otherSpec})[key] {
		t.Fatal("dynamic project directory authorized exclusion")
	}
}

func TestFLPV1007IntentOnlySuiteSurvivesOtherInventory(t *testing.T) {
	f := flowPlanRepo(t, false)
	set, err := LoadIntents(f.root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	checkout := set.Flows[0]
	checkout.Links[0].Target.Path = "lib/lib.go"
	writeIntent(t, f.root, "flows/checkout.json", checkout)
	checkout.Links[0].ReviewedAt = commitAll(t, f.root, "checkout uses Go")
	writeIntent(t, f.root, "flows/checkout.json", checkout)
	if err := os.Remove(filepath.Join(f.root, "playwright.config.ts")); err != nil {
		t.Fatal(err)
	}
	f.head = commitAll(t, f.root, "review checkout source")
	_, control := f.plan(t, "lib/lib.go")
	expected, ok := e2eEntry(control, cartFile)
	if !ok || !slices.Contains(expected.Reasons, affected.E2EReasonFlowReach) {
		t.Fatalf("intent control %+v", control.E2E)
	}
	lister, _ := testinventory.CurrentLister(testid.Go)
	body, err := testinventory.Encode(testinventory.Inventory{Profile: testinventory.Profile, Instances: []testinventory.Instance{{
		Key: "go@.", Root: ".", Runner: testid.Go, Config: "go.mod", ConfigDigest: testinventory.BlobID("sha1", []byte(readFile(t, f.root, "go.mod"))),
		Commit: f.head, Lister: testinventory.Lister{Name: lister.Name, Version: lister.Version, Kind: lister.Kind},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		inventory []byte
	}{
		{"Go inventory", body}, {"unknown inventory", []byte(`{"profile":"future-inventory/9"}`)},
	} {
		writeRaw(t, f.root, testinventory.Path, tc.inventory)
		commitAll(t, f.root, tc.name)
		_, plan := f.plan(t, "lib/lib.go")
		got, ok := e2eEntry(plan, cartFile)
		if !ok || !reflect.DeepEqual(got, expected) {
			t.Fatalf("%s lost intent obligation: %+v", tc.name, plan.E2E)
		}
		if tc.name == "Go inventory" && !reflect.DeepEqual(control.E2E, plan.E2E) {
			t.Fatalf("Go inventory changed intent selection: %+v, want %+v", plan.E2E, control.E2E)
		}
	}
}

func TestFLPV1007StatusKeepsReviewedIntentResults(t *testing.T) {
	f := linkFixture(t)
	set, err := LoadIntents(f.root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	set.Flows[0].Links[1].Target.TestKey = linkedCart
	for _, flow := range set.Flows {
		for i := range flow.Links {
			flow.Links[i].ReviewedAt = ""
		}
		writeIntent(t, f.root, set.IntentPath(flow.FlowID), flow)
	}
	anchor := commitAll(t, f.root, "declare current checkout and search targets")
	for _, flow := range set.Flows {
		for i := range flow.Links {
			flow.Links[i].ReviewedAt = anchor
		}
		writeIntent(t, f.root, set.IntentPath(flow.FlowID), flow)
	}
	commitAll(t, f.root, "review current checkout and search targets")
	acceptCart(t, f, linkedCart)
	set, err = LoadIntentsAt(t.Context(), f.root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	record := TestRunEvidence{TestKey: "search.spec.ts > finds", Project: "chromium", RunID: "search-run", Attempts: []RunAttempt{{Outcome: "failed"}}}
	raw, err := FlowStatus(t.Context(), f.root, set, StatusOptions{Results: Results{Records: []TestRunEvidence{record}}})
	var status StatusReport
	if err != nil || json.Unmarshal(raw, &status) != nil {
		t.Fatalf("status %s: %v", raw, err)
	}
	checkout := status.Flows[0].Variations[0]
	search := status.Flows[1].Variations[0]
	if len(checkout.Tests) != 1 || checkout.Tests[0].TestKey != linkedCart || checkout.Tests[0].Reason != "" ||
		status.Totals.Tests != 3 || search.Code != "" || len(search.Tests) != 1 || search.Tests[0].TestKey != record.TestKey ||
		search.Tests[0].Result != "failed" || search.Tests[0].RunID != record.RunID {
		t.Fatalf("merged status %s", raw)
	}
}
