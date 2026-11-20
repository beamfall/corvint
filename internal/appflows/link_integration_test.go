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
	// The search spec owns this import, so its helper is provably excludable
	// from checkout even though both files run in the browser suite.
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
	result := plan("src/search.ts")
	before, ok := e2eEntry(result, cartFile)
	if !ok || !slices.Contains(before.Reasons, "unassigned") || !slices.Equal(before.TestKeys, []string{linkedCart, otherCart}) {
		t.Fatalf("tag alone: %+v", result.E2E)
	}
	if _, err := ProposeLinks(t.Context(), f.root, "", true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.E2E, plan("src/search.ts").E2E) {
		t.Fatal("proposals altered selection")
	}
	acceptCart(t, f, linkedCart, pyCart)
	after := plan("src/search.ts")
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
	control := JoinFlowPlan(t.Context(), f.root, graph, affected.Select(graph, []string{"src/search.ts"}), "")
	row, ok := e2eEntry(control, cartFile)
	if !ok || !slices.Equal(row.TestKeys, []string{otherCart}) {
		t.Fatal("the search-only helper has a closed exclusion", row)
	}
	for _, tc := range []struct {
		name  string
		dirty []string
	}{
		{"browser route", []string{"server/checkout.go"}},
		{"mixed paths", []string{"src/search.ts", "server/checkout.go"}},
		{"future frontier", []string{"src/search.ts"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := affected.Select(graph, tc.dirty)
			if tc.name == "future frontier" {
				plan.Unknown = []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: "future:runtime-binding:src/search.ts"}}
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
