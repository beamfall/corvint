// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// reportProvider writes one application-flow-selection-provider/1 file into the fixture root and
// returns its repository-relative name.
func reportProvider(t *testing.T, root, name, flows, discovery string, inventory ...appflows.InventoryTest) string {
	t.Helper()
	p := appflows.SelectionProvider{Schema: appflows.SelectionProviderSchema, Flows: flows, RunnerConfig: "playwright.config.ts",
		Discovery: discovery, GlobalPaths: []string{}, Inventory: inventory}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{name: string(raw) + "\n"})
	return name
}

func inventoryTest(key, project, path string) appflows.InventoryTest {
	return appflows.InventoryTest{TestKey: key, Project: project, Path: path}
}

// reportReceipt writes a canonical playwright-discovery/0 receipt bound to revision with the given
// config digest and source digest; a receipt bound to another revision is what makes an inventory
// stale (AFU-V1-153).
func reportReceipt(t *testing.T, root, name, revision, configSHA, sourceDigest string, units ...typescript.PlaywrightDiscoveryUnit) {
	t.Helper()
	receipt := typescript.PlaywrightDiscovery{Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: configSHA},
		Profile: "playwright-discovery/0", Revision: revision, SourceDigest: sourceDigest, Units: units}
	raw, err := jsonv2.Marshal(receipt, jsonv2.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{name: string(raw) + "\n"})
}

func decodeGaps(t *testing.T, root string, args ...string) appflows.GapsReport {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(root, append([]string{"gaps", "--flows", "flows"}, args...)...)
	if code != 0 {
		t.Fatalf("gaps %v exited %d: %s", args, code, diagnostic)
	}
	var report appflows.GapsReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func unlinkedKeys(report appflows.GapsReport) []string {
	keys := []string{}
	for _, g := range report.UnlinkedTests {
		if g.Code != appflows.GapUnlinkedTest {
			keys = append(keys, "WRONG-CODE:"+g.Code)
		}
		keys = append(keys, g.TestKey)
	}
	return keys
}

// AFU-V1-150 AFU-V1-153 (V1-5255): without a provider the inventory is absent and nothing is
// claimed; with one, every inventory test no variation links with a non-inferred test link is
// listed, in test-key order, and the inventory state is complete, unverified, stale or partial.
func TestV15255GapsListsUnlinkedInventoryTests(t *testing.T) {
	fx := newShopFixture(t)
	absent := decodeGaps(t, fx.root, "--evidence", fx.evidence)
	if absent.Inventory.State != appflows.InventoryAbsent || len(absent.UnlinkedTests) != 0 || absent.UnlinkedTotal != 0 || absent.UnlinkedTruncated || !strings.Contains(absent.Inventory.Reason, "no --provider") {
		t.Fatalf("no provider: %+v %+v", absent.Inventory, absent.UnlinkedTests)
	}
	for _, f := range absent.Flows {
		for _, g := range f.Gaps {
			if g.Code == appflows.GapUnlinkedTest {
				t.Fatalf("%s carries %s inside its gaps", f.FlowID, g.Code)
			}
		}
	}

	// One unlinked test, one test with only inferred links, two linked tests.
	provider := reportProvider(t, fx.root, "e2e/provider.json", "flows", "",
		inventoryTest("wishlist.spec.ts > adds", "", "e2e/wishlist.spec.ts"),
		inventoryTest("checkout.spec.ts > pays", "chromium", "e2e/checkout.spec.ts"),
		inventoryTest("checkout.spec.ts > declines", "chromium", "e2e/checkout.spec.ts"),
		inventoryTest("search.spec.ts > finds", "chromium", "e2e/search.spec.ts"))
	report := decodeGaps(t, fx.root, "--evidence", fx.evidence, "--provider", provider)
	if want := []string{"checkout.spec.ts > declines", "wishlist.spec.ts > adds"}; !slices.Equal(unlinkedKeys(report), want) || report.UnlinkedTotal != 2 || report.UnlinkedTruncated {
		t.Fatalf("unlinked: %+v total=%d truncated=%v", report.UnlinkedTests, report.UnlinkedTotal, report.UnlinkedTruncated)
	}
	if report.UnlinkedTests[0].Project != "chromium" || !strings.Contains(report.UnlinkedTests[0].Detail, "no flow variation links this test") || !strings.Contains(report.UnlinkedTests[0].Detail, "e2e/checkout.spec.ts") {
		t.Errorf("declines: %+v", report.UnlinkedTests[0])
	}
	if !strings.Contains(report.UnlinkedTests[1].Detail, "only inferred test links") {
		t.Errorf("adds: %+v", report.UnlinkedTests[1])
	}
	if report.Inventory.State != appflows.InventoryComplete || report.Inventory.Source != "provider:"+provider || report.Inventory.Tests != 4 || !strings.Contains(report.Inventory.Reason, "attestation") {
		t.Errorf("inventory: %+v", report.Inventory)
	}
	// The flow gaps themselves are unchanged by the provider.
	if a, b := flowGapsOnly(absent), flowGapsOnly(report); a != b {
		t.Errorf("flow gaps changed with a provider:\n%s\n%s", a, b)
	}

	// Every test linked: an empty list under a complete inventory.
	linked := reportProvider(t, fx.root, "e2e/linked.json", "flows", "",
		inventoryTest("checkout.spec.ts > pays", "chromium", "e2e/checkout.spec.ts"),
		inventoryTest("profile.spec.ts > edits", "", "e2e/profile.spec.ts"))
	if report = decodeGaps(t, fx.root, "--provider", linked); len(report.UnlinkedTests) != 0 || report.UnlinkedTotal != 0 || report.Inventory.State != appflows.InventoryComplete || report.Inventory.Tests != 2 {
		t.Errorf("all linked: %+v %+v", report.Inventory, report.UnlinkedTests)
	}

	// A declared discovery receipt that is missing leaves the inventory unverified, not complete.
	unverified := reportProvider(t, fx.root, "e2e/unverified.json", "flows", "e2e/discovery.json",
		inventoryTest("checkout.spec.ts > declines", "chromium", "e2e/checkout.spec.ts"))
	if report = decodeGaps(t, fx.root, "--provider", unverified); report.Inventory.State != appflows.InventoryUnverified || !strings.Contains(report.Inventory.Reason, "MISSING") || len(report.UnlinkedTests) != 1 {
		t.Errorf("missing receipt: %+v %d", report.Inventory, len(report.UnlinkedTests))
	}

	// A receipt bound to commit B is stale at HEAD and names B.
	digest := strings.Repeat("a", 64)
	reportReceipt(t, fx.root, "e2e/discovery.json", fx.b, digest, "playwright-sources:sha256:"+digest,
		typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/checkout.spec.ts"})
	if report = decodeGaps(t, fx.root, "--provider", unverified); report.Inventory.State != appflows.InventoryStale || report.Inventory.BoundTo != fx.b || !strings.Contains(report.Inventory.Reason, fx.b) || len(report.UnlinkedTests) != 1 {
		t.Errorf("stale receipt: %+v", report.Inventory)
	}

	// A receipt bound to HEAD's config and sources that lists a test the inventory lacks is partial.
	shopWrite(t, fx.root, map[string]string{"playwright.config.ts": "import { defineConfig } from '@playwright/test';\nexport default defineConfig({ testDir: './e2e', projects: [{ name: 'chromium' }] });\n"})
	configRaw, err := os.ReadFile(filepath.Join(fx.root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	configSum := sha256.Sum256(configRaw)
	sourceDigest, err := typescript.ObservePlaywrightSources(fx.root, "playwright.config.ts")
	if err != nil {
		t.Fatalf("observe sources: %v", err)
	}
	reportReceipt(t, fx.root, "e2e/discovery.json", fx.head, hex.EncodeToString(configSum[:]), sourceDigest,
		typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/checkout.spec.ts"},
		typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/search.spec.ts"})
	report = decodeGaps(t, fx.root, "--provider", unverified)
	if report.Inventory.State != appflows.InventoryPartial || !strings.Contains(report.Inventory.Reason, "not-inventoried:chromium:e2e/search.spec.ts") {
		t.Errorf("partial receipt: %+v", report.Inventory)
	}
	// The same receipt with exactly the inventory is complete through discovery.
	reportReceipt(t, fx.root, "e2e/discovery.json", fx.head, hex.EncodeToString(configSum[:]), sourceDigest,
		typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/checkout.spec.ts"})
	if report = decodeGaps(t, fx.root, "--provider", unverified); report.Inventory.State != appflows.InventoryComplete || !strings.Contains(report.Inventory.Reason, "lists exactly the inventory") {
		t.Errorf("matched receipt: %+v", report.Inventory)
	}

	// A provider for another flows directory is refused whole, as is one that is not a provider.
	other := reportProvider(t, fx.root, "e2e/other.json", "other-flows", "", inventoryTest("x", "", "e2e/x.spec.ts"))
	if code, out, diagnostic := runFlowsCLI(fx.root, "gaps", "--flows", "flows", "--provider", other); code != 2 || out != "" || !strings.Contains(diagnostic, "invalid-arguments") || !strings.Contains(diagnostic, "other-flows") {
		t.Errorf("mismatched provider: %d %q %s", code, out, diagnostic)
	}
	if code, out, _ := runFlowsCLI(fx.root, "gaps", "--flows", "flows", "--provider", "e2e/discovery.json"); code != 2 || out != "" {
		t.Errorf("non-provider: %d %q", code, out)
	}
	if code, out, _ := runFlowsCLI(fx.root, "gaps", "--flows", "flows", "--provider", "e2e/absent.json"); code != 2 || out != "" {
		t.Errorf("absent provider file: %d %q", code, out)
	}
}

func flowGapsOnly(report appflows.GapsReport) string {
	raw, _ := json.Marshal(report.Flows)
	return string(raw)
}

// AFU-V1-152 (V1-5255): more unlinked tests than the bound gives exactly the bound, in stable order,
// with the truncation marker and the total.
func TestV15255GapsUnlinkedBound(t *testing.T) {
	fx := newShopFixture(t)
	inventory := []appflows.InventoryTest{}
	for i := appflows.MaxUnlinkedTests + 6; i > 0; i-- {
		inventory = append(inventory, inventoryTest(fmt.Sprintf("bulk.spec.ts > case %05d", i), "chromium", "e2e/bulk.spec.ts"))
	}
	provider := reportProvider(t, fx.root, "e2e/bulk.json", "flows", "", inventory...)
	report := decodeGaps(t, fx.root, "--provider", provider)
	if len(report.UnlinkedTests) != appflows.MaxUnlinkedTests || !report.UnlinkedTruncated || report.UnlinkedTotal != appflows.MaxUnlinkedTests+6 {
		t.Fatalf("bound: %d entries, truncated %v, total %d", len(report.UnlinkedTests), report.UnlinkedTruncated, report.UnlinkedTotal)
	}
	if first, last := report.UnlinkedTests[0].TestKey, report.UnlinkedTests[appflows.MaxUnlinkedTests-1].TestKey; first != "bulk.spec.ts > case 00001" || last != fmt.Sprintf("bulk.spec.ts > case %05d", appflows.MaxUnlinkedTests) {
		t.Fatalf("order: %s .. %s", first, last)
	}
}

func decodeStatus(t *testing.T, root string, args ...string) (appflows.StatusReport, string) {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(root, append([]string{"status", "--flows", "flows"}, args...)...)
	if code != 0 {
		t.Fatalf("status %v exited %d: %s", args, code, diagnostic)
	}
	var report appflows.StatusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	return report, out
}

func statusVariation(t *testing.T, report appflows.StatusReport, flow, variation string) appflows.StatusVariation {
	t.Helper()
	for _, f := range report.Flows {
		if f.FlowID != flow {
			continue
		}
		for _, v := range f.Variations {
			if v.VariationID == variation {
				return v
			}
		}
	}
	t.Fatalf("%s/%s not in the report", flow, variation)
	return appflows.StatusVariation{}
}

func statusFlowIDs(report appflows.StatusReport) []string {
	ids := []string{}
	for _, f := range report.Flows {
		ids = append(ids, f.FlowID)
	}
	slices.Sort(ids)
	return ids
}

// AFU-V1-160 AFU-V1-161 AFU-V1-164 (V1-5257): one report shows every flow, variation, attached test
// and last result; a never-run test is NOT_OBSERVED, a run on an older commit is stale with that
// commit, a variation with no test carries no-test, and the report is deterministic and read-only.
func TestV15257StatusReportsAttachedTestsAndLastResults(t *testing.T) {
	fx := newShopFixture(t)
	before := flowSnapshot(t, fx.root)
	report, out := decodeStatus(t, fx.root, "--evidence", fx.evidence)
	if report.Schema != appflows.StatusSchema || report.Revision != fx.head || report.LimitedByBase || report.Base != "" || report.ImpactGraph != nil || report.Truncated {
		t.Fatalf("header: %+v", report)
	}
	if want := []string{"checkout", "profile", "returns", "search", "wishlist"}; !slices.Equal(statusFlowIDs(report), want) {
		t.Fatalf("flows %v", statusFlowIDs(report))
	}
	if report.Totals != (appflows.StatusTotals{Flows: 5, Variations: 6, Tests: 6}) {
		t.Errorf("totals %+v", report.Totals)
	}
	happy := statusVariation(t, report, "checkout", "checkout.happy")
	if len(happy.Tests) != 1 || happy.Code != "" || happy.Tests[0] != (appflows.StatusTest{TestKey: "checkout.spec.ts > pays", Project: "chromium", Result: "passed", SourceCommit: fx.head, RunID: "run-1", Authority: appflows.AuthorityIngested}) {
		t.Errorf("checkout.happy: %+v", happy)
	}
	basic := statusVariation(t, report, "search", "search.basic")
	results := []string{}
	for _, test := range basic.Tests {
		results = append(results, test.Project+"="+test.Result)
	}
	if want := []string{"chromium=flaky", "firefox=passed", "webkit=passed"}; !slices.Equal(results, want) || !slices.Equal(basic.Projects, []string{"chromium", "firefox", "webkit"}) {
		t.Errorf("search.basic: %v %v", results, basic.Projects)
	}
	edit := statusVariation(t, report, "profile", "profile.edit")
	if len(edit.Tests) != 1 || !edit.Tests[0].Stale || edit.Tests[0].Result != "passed" || edit.Tests[0].SourceCommit != fx.b || edit.Tests[0].Project != "" {
		t.Errorf("profile.edit: %+v", edit.Tests)
	}
	view := statusVariation(t, report, "profile", "profile.view")
	if len(view.Tests) != 1 || view.Tests[0] != (appflows.StatusTest{TestKey: "profile.spec.ts > views", Result: appflows.ResultNotObserved}) {
		t.Errorf("profile.view (STATIC record never counts): %+v", view.Tests)
	}
	for _, name := range [][2]string{{"returns", "returns.basic"}, {"wishlist", "wishlist.add"}} {
		if v := statusVariation(t, report, name[0], name[1]); v.Code != appflows.GapNoTest || len(v.Tests) != 0 {
			t.Errorf("%s: %+v", name[1], v)
		}
	}
	// No evidence at all: every result is NOT_OBSERVED, which is an answer, not a failure.
	bare, _ := decodeStatus(t, fx.root)
	for _, f := range bare.Flows {
		for _, v := range f.Variations {
			for _, test := range v.Tests {
				if test.Result != appflows.ResultNotObserved || test.Stale || test.SourceCommit != "" {
					t.Errorf("%s without evidence: %+v", v.VariationID, test)
				}
			}
		}
	}
	// Deterministic and read-only.
	if _, again := decodeStatus(t, fx.root, "--evidence", fx.evidence); again != out {
		t.Error("two runs differ")
	}
	after := flowSnapshot(t, fx.root)
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d to %d", len(before), len(after))
	}
	for p, sum := range before {
		if after[p] != sum {
			t.Fatalf("%s changed", p)
		}
	}
}

// AFU-V1-163 AFU-V1-165 (V1-5257): --base limits the report to exactly the flows `flows impact`
// names and says so; an unresolvable base, unreadable evidence, --registry and a malformed flow
// file are refused whole with exit 2 and no report.
func TestV15257StatusLimitedToChangeAndRefusals(t *testing.T) {
	fx := newShopFixture(t)
	code, out, diagnostic := runFlowsCLI(fx.root, "impact", "--flows", "flows", "--base", fx.b)
	if code != 0 {
		t.Fatalf("impact exited %d: %s", code, diagnostic)
	}
	var impact appflows.ImpactReport
	if err := json.Unmarshal([]byte(out), &impact); err != nil {
		t.Fatal(err)
	}
	reached := []string{}
	for _, f := range impact.Flows {
		reached = append(reached, f.FlowID)
	}
	slices.Sort(reached)
	if len(reached) == 0 || len(reached) == 5 {
		t.Fatalf("the fixture change must reach some flows and not all: %v", reached)
	}
	report, _ := decodeStatus(t, fx.root, "--evidence", fx.evidence, "--base", fx.b)
	if !slices.Equal(statusFlowIDs(report), reached) || !report.LimitedByBase || report.Base != fx.b || report.ImpactGraph == nil || report.ImpactGraph.Scope != impact.Graph.Scope || report.Totals.Flows != len(reached) {
		t.Fatalf("limited report %v (impact %v): %+v", statusFlowIDs(report), reached, report)
	}

	zero := strings.Repeat("0", 40)
	if code, out, diagnostic := runFlowsCLI(fx.root, "status", "--flows", "flows", "--base", zero); code != 2 || out != "" || !strings.Contains(diagnostic, "unsupported-affected-revision") {
		t.Errorf("unknown base: %d %q %s", code, out, diagnostic)
	}
	if code, out, diagnostic := runFlowsCLI(fx.root, "status", "--flows", "flows", "--evidence", filepath.Join(fx.root, "missing.jsonl")); code != 2 || out != "" || !strings.Contains(diagnostic, "invalid-arguments") {
		t.Errorf("unreadable evidence: %d %q %s", code, out, diagnostic)
	}
	for _, args := range [][]string{{"status"}, {"status", "--flows", "flows", "--registry", "runs/registry.json"}, {"status", "--flows", "flows", "extra"}, {"status", "--flows", "absent"}} {
		if code, out, _ := runFlowsCLI(fx.root, args...); code != 2 || out != "" {
			t.Errorf("%v: %d %q", args, code, out)
		}
	}
	shopWrite(t, fx.root, map[string]string{"flows/broken.json": "{\"schema\":\"application-flow-intent/1\",\"flow_id\":\"broken\""})
	shopCommit(t, fx.root, "D: malformed flow")
	if code, out, diagnostic := runFlowsCLI(fx.root, "status", "--flows", "flows"); code != 2 || out != "" || !strings.Contains(diagnostic, "broken") {
		t.Errorf("malformed flow: %d %q %s", code, out, diagnostic)
	}
}

// AFU-V1-164 (V1-5257): more attached test rows than the bound drops rows in report order with the
// truncation marker and the full totals.
func TestV15257StatusBound(t *testing.T) {
	fx := newShopFixture(t)
	projects, links := []string{}, []appflows.FlowLink{shopLink("fill", "declared", fx.head, appflows.LinkTarget{Type: "source", Path: "cart/cart.go"})}
	for i := 0; i < 33; i++ {
		projects = append(projects, fmt.Sprintf("p%02d", i))
	}
	for i := 0; i < 256; i++ {
		links = append(links, shopLink("bulk.all", "declared", fx.head, appflows.LinkTarget{Type: "test", Path: "e2e/checkout.spec.ts", TestKey: fmt.Sprintf("checkout.spec.ts > bulk %03d", i)}))
	}
	intent := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: "bulk", Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
		Steps: []appflows.FlowStep{{StepID: "fill", Action: "fill the cart"}}, Outcomes: []appflows.FlowOutcome{{OutcomeID: "full", Behavior: "cart full", Matcher: "toBeVisible", Locator: "#full", Value: "full"}},
		Variations: []appflows.FlowVariation{{VariationID: "bulk.all", Preconditions: []string{}, Steps: []string{"fill"}, ObservableFacts: []string{}, Outcomes: []string{"full"}, Projects: projects}},
		Links:      links}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, fx.root, map[string]string{"flows/bulk.json": string(raw)})
	shopCommit(t, fx.root, "D: bulk flow")
	report, _ := decodeStatus(t, fx.root)
	rows := 0
	for _, f := range report.Flows {
		for _, v := range f.Variations {
			rows += len(v.Tests)
		}
	}
	if !report.Truncated || rows != appflows.MaxStatusTests || report.Totals.Tests != 33*256+6 || report.Totals.Flows != 6 {
		t.Fatalf("bound: truncated %v rows %d totals %+v", report.Truncated, rows, report.Totals)
	}
}

// AFU-V1-151 AFU-V1-162 (V1-5255, V1-5257): the gaps tool with a provider and the status tool return,
// as their receipts, the documents the CLI verbs write for the same inputs; a bad provider path or
// base is invalid-arguments, a provider for another directory is flows-refused.
func TestV15255V15257FlowsToolsMatchCLIVerbs(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the repository probe is qualified only on Darwin and Linux")
	}
	fx := newShopFixture(t)
	fx.root = resolvedRoot(t, fx.root)
	registry := flowsRegistry(t, fx.root)
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsStatus, map[string]any{"flows": "flows", "base": fx.b}, "status", "--flows", "flows", "--base", fx.b)
	evidence := copyIntoRoot(t, fx.root, fx.evidence, "runs/runs.jsonl")
	provider := reportProvider(t, fx.root, "e2e/provider.json", "flows", "e2e/discovery.json",
		inventoryTest("checkout.spec.ts > declines", "chromium", "e2e/checkout.spec.ts"),
		inventoryTest("checkout.spec.ts > pays", "chromium", "e2e/checkout.spec.ts"))
	digest := strings.Repeat("b", 64)
	reportReceipt(t, fx.root, "e2e/discovery.json", fx.b, digest, "playwright-sources:sha256:"+digest)
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsGaps, map[string]any{"flows": "flows", "evidence": []any{"runs/runs.jsonl"}, "provider": provider}, "gaps", "--flows", "flows", "--evidence", evidence, "--provider", provider)
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsGaps, map[string]any{"flows": "flows"}, "gaps", "--flows", "flows")
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsStatus, map[string]any{"flows": "flows", "evidence": []any{"runs/runs.jsonl"}}, "status", "--flows", "flows", "--evidence", evidence)
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsStatus, map[string]any{"flows": "flows", "evidence": []any{"runs/runs.jsonl"}, "base": fx.b}, "status", "--flows", "flows", "--evidence", evidence, "--base", fx.b)

	result, callErr := registry.Call(t.Context(), bridge.ToolFlowsGaps, []byte(`{"flows":"flows","provider":"`+provider+`"}`))
	if callErr != nil || result.Receipt["inventory"].(map[string]any)["state"] != appflows.InventoryStale {
		t.Errorf("stale inventory over MCP: %#v %#v", callErr, result.Receipt["inventory"])
	}
	for name, call := range map[string]struct{ tool, arguments string }{
		"absolute provider": {bridge.ToolFlowsGaps, `{"flows":"flows","provider":"/e2e/provider.json"}`},
		"git provider":      {bridge.ToolFlowsGaps, `{"flows":"flows","provider":".git/config"}`},
		"short base":        {bridge.ToolFlowsStatus, `{"flows":"flows","base":"abc"}`},
		"unknown member":    {bridge.ToolFlowsStatus, `{"flows":"flows","registry":"runs/registry.json"}`},
	} {
		if _, err := registry.Call(t.Context(), call.tool, []byte(call.arguments)); err == nil || err.Code != "invalid-arguments" {
			t.Errorf("%s: %#v", name, err)
		}
	}
	other := reportProvider(t, fx.root, "e2e/other.json", "elsewhere", "", inventoryTest("x", "", "e2e/x.spec.ts"))
	if _, err := registry.Call(t.Context(), bridge.ToolFlowsGaps, []byte(`{"flows":"flows","provider":"`+other+`"}`)); err == nil || err.Code != "flows-refused" {
		t.Errorf("mismatched provider: %#v", err)
	}
	if _, err := registry.Call(t.Context(), bridge.ToolFlowsStatus, []byte(`{"flows":"flows","base":"`+strings.Repeat("0", 40)+`"}`)); err == nil || err.Code != "flows-refused" {
		t.Errorf("unknown base: %#v", err)
	}
}
