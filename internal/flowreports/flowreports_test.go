// SPDX-License-Identifier: Apache-2.0

package flowreports

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and system
// configuration (testsupport.IsolateGit, V1-2635).
func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	os.Exit(m.Run())
}

// shop is the fixture repository: two flows, checkout (one variation, one test) and search (two
// variations, one with a test and one with none). A adds the sources and intents, B reviews the
// links at A, and HEAD (C) changes src/checkout.ts, which the checkout step links.
type shop struct {
	root, a, b, head, tree string
}

const (
	checkoutKey = "checkout.spec.ts > pays"
	searchKey   = "search.spec.ts > finds"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	command.Dir = root
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func commit(t *testing.T, root, message string) string {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", message)
	return git(t, root, "rev-parse", "HEAD")
}

func intents(t *testing.T, anchor string) map[string]string {
	t.Helper()
	flow := func(id, step, source string, variations ...appflows.FlowVariation) appflows.FlowIntent {
		intent := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: id, Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
			Steps:      []appflows.FlowStep{{StepID: step, Action: "do " + step}},
			Outcomes:   []appflows.FlowOutcome{{OutcomeID: step + "-done", Behavior: "done", Matcher: "toBeVisible", Locator: "#" + step, Value: step}},
			Variations: variations,
			Links:      []appflows.FlowLink{{From: step, Basis: "declared", Target: appflows.LinkTarget{Type: "source", Path: source}, ReviewedAt: anchor}}}
		return intent
	}
	variation := func(id, step string) appflows.FlowVariation {
		return appflows.FlowVariation{VariationID: id, Preconditions: []string{}, Steps: []string{step}, ObservableFacts: []string{}, Outcomes: []string{step + "-done"}, Projects: []string{"chromium"}}
	}
	testLink := func(from, path, key string) appflows.FlowLink {
		return appflows.FlowLink{From: from, Basis: "declared", Target: appflows.LinkTarget{Type: "test", Path: path, TestKey: key}, ReviewedAt: anchor}
	}
	checkout := flow("checkout", "pay", "src/checkout.ts", variation("checkout.basic", "pay"))
	checkout.Links = append(checkout.Links, testLink("checkout.basic", "e2e/checkout.spec.ts", checkoutKey))
	search := flow("search", "query", "src/search.ts", variation("search.basic", "query"), variation("search.none", "query"))
	search.Links = append(search.Links, testLink("search.basic", "e2e/search.spec.ts", searchKey))
	out := map[string]string{}
	for _, intent := range []appflows.FlowIntent{checkout, search} {
		raw, err := json.Marshal(intent)
		if err != nil {
			t.Fatal(err)
		}
		out["flows/"+intent.FlowID+".json"] = string(raw) + "\n"
	}
	return out
}

func newShop(t *testing.T) shop {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	write(t, root, map[string]string{
		"e2e/checkout.spec.ts":        "test(\"pays\", () => {});\n",
		"e2e/search.spec.ts":          "test(\"finds\", () => {});\n",
		"src/checkout.ts":             "export function pay(n: number): number {\n  return n;\n}\n",
		"src/search.ts":               "export function find(q: string): string {\n  return q;\n}\n",
		appflows.FlowsDeclarationPath: `{"profile":"` + appflows.FlowsProfile(appflows.FlowsWriteVersion()) + `","flows":"flows"}` + "\n",
	})
	write(t, root, intents(t, ""))
	fx := shop{root: root}
	fx.a = commit(t, root, "A: shop")
	write(t, root, intents(t, fx.a))
	fx.b = commit(t, root, "B: review at A")
	write(t, root, map[string]string{"src/checkout.ts": "export function pay(n: number): number {\n  return n + 1;\n}\n"})
	fx.head = commit(t, root, "C: change checkout")
	fx.tree = git(t, root, "rev-parse", "HEAD^{tree}")
	return fx
}

// testEnv is a JSON error envelope like the binaries' own.
var testEnv = Env{
	EmitError: func(diagnostic io.Writer, err error) {
		code := "internal-error"
		var coded *gokernel.Error
		if errors.As(err, &coded) {
			code = coded.Code
		}
		raw, _ := json.Marshal(map[string]any{"code": code, "error": err.Error(), "ok": false})
		fmt.Fprintln(diagnostic, string(raw))
	},
	BaseRefusal: func(base string) error {
		return &gokernel.Error{Code: "unsupported-affected-revision", Message: "base " + base + " is not a commit in this repository"}
	},
}

func report(t *testing.T, root, verb string, args ...string) (int, string, string) {
	t.Helper()
	var out, diagnostic bytes.Buffer
	code := Run(context.Background(), root, verb, args, &out, &diagnostic, testEnv)
	return code, out.String(), diagnostic.String()
}

func status(t *testing.T, root string, args ...string) (appflows.StatusReport, string) {
	t.Helper()
	code, out, diagnostic := report(t, root, "status", append([]string{"--flows", "flows"}, args...)...)
	if code != 0 || diagnostic != "" {
		t.Fatalf("flows status %v: %d %s", args, code, diagnostic)
	}
	var r appflows.StatusReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	return r, out
}

func row(t *testing.T, r appflows.StatusReport, flow, variation string) []appflows.StatusTest {
	t.Helper()
	for _, f := range r.Flows {
		for _, v := range f.Variations {
			if f.FlowID == flow && v.VariationID == variation {
				return v.Tests
			}
		}
	}
	t.Fatalf("no %s/%s in %+v", flow, variation, r.Flows)
	return nil
}

// receipt is a corvint-test-run/0 receipt of one Playwright file unit, as corvint test writes it.
func receipt(t *testing.T, head, tree, outcome, file string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"profile": "corvint-test-run/0",
		"source":  map[string]any{"head": head, "tree": tree, "paths": []any{}},
		"invocations": []any{map[string]any{"adapter": "typescript", "version": "1.50.0", "outcomes": []any{
			map[string]any{"outcome": outcome, "unit": "typescript:playwright:" + file, "testId": "playwright:" + file},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// ingested is one test-run-evidence/0 line flows ingest writes for one title.
func ingested(t *testing.T, key, commit, tree string, outcome string) []byte {
	t.Helper()
	digest := strings.Repeat("a", 64)
	line, err := appflows.EncodeRunEvidence(appflows.TestRunEvidence{Schema: appflows.RunEvidenceSchema, Authority: appflows.AuthorityIngested, RunID: "run-1",
		Runner: appflows.RunRunner{Name: "playwright", Version: "1.50.0"}, Source: appflows.RunSource{Commit: commit, Tree: tree, Clean: true},
		BuildArtifactDigest: digest, Environment: appflows.RunDigestRef{ID: "ci", Digest: digest}, Fixture: appflows.RunDigestRef{ID: "seed", Digest: digest},
		TestKey: key, Project: "chromium", Cleanup: "done", NegativeControls: []appflows.NegativeControl{},
		Attempts: []appflows.RunAttempt{{Ordinal: 1, Outcome: outcome, AssertionAnchors: []appflows.RunAnchor{}, Attachments: []appflows.RunAttachment{}}}})
	if err != nil {
		t.Fatal(err)
	}
	return line
}

func file(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// AFL-V1-001: every flow and variation with its linked tests; a variation with no test says so;
// without a provider the unassigned count is null and the inventory absent.
func TestAFLV1001StatusListsFlowsVariationsAndTests_V15623(t *testing.T) {
	fx := newShop(t)
	r, _ := status(t, fx.root, "--no-store")
	var flows []string
	for _, f := range r.Flows {
		var variations []string
		for _, v := range f.Variations {
			variations = append(variations, v.VariationID+"="+fmt.Sprint(len(v.Tests))+v.Code)
		}
		flows = append(flows, f.FlowID+":"+strings.Join(variations, ","))
	}
	if want := []string{"checkout:checkout.basic=1", "search:search.basic=1,search.none=0" + appflows.GapNoTest}; !slices.Equal(flows, want) {
		t.Fatalf("flows %v, want %v", flows, want)
	}
	if r.Revision != fx.head || r.Totals != (appflows.StatusTotals{Flows: 2, Variations: 3, Tests: 2}) || r.Truncated {
		t.Fatalf("revision %s totals %+v truncated %v", r.Revision, r.Totals, r.Truncated)
	}
	if r.Unassigned != nil || r.Inventory.State != appflows.InventoryAbsent {
		t.Fatalf("unassigned %v inventory %+v without a provider", r.Unassigned, r.Inventory)
	}
}

// AFL-V1-002: no result source at all: every result NOT_OBSERVED, exit 0, and the sources it
// looked at are listed: the empty store and the absent default receipt.
func TestAFLV1002NoSourceIsNotObserved_V15623(t *testing.T) {
	fx := newShop(t)
	r, _ := status(t, fx.root)
	for _, key := range [][2]string{{"checkout", "checkout.basic"}, {"search", "search.basic"}} {
		if got := row(t, r, key[0], key[1]); len(got) != 1 || got[0].Result != appflows.ResultNotObserved || got[0].Granularity != "" || got[0].Stale {
			t.Fatalf("%v: %+v", key, got)
		}
	}
	want := []appflows.ResultSource{{Kind: appflows.SourceStore, State: appflows.SourceRead}, {Kind: appflows.SourceReceipt, Path: appflows.DefaultReceiptLabel, State: appflows.SourceAbsent}}
	if !slices.Equal(r.ResultSources, want) {
		t.Fatalf("result_sources %+v, want %+v", r.ResultSources, want)
	}
	r, _ = status(t, fx.root, "--no-store")
	if len(r.ResultSources) != 2 || r.ResultSources[0].State != appflows.SourceSkipped || r.ResultSources[1].State != appflows.SourceSkipped {
		t.Fatalf("--no-store sources %+v", r.ResultSources)
	}
}

// AFL-V1-002: a corvint test receipt names a test file; its outcome reaches the title link of that
// file with granularity file, never as a per-title result, and the test the receipt does not
// cover stays NOT_OBSERVED. The default receipt is read the same way.
func TestAFLV1002ReceiptGivesFileLevelOutcome_V15623(t *testing.T) {
	fx := newShop(t)
	named := file(t, "test-run.json", receipt(t, fx.head, fx.tree, "FAIL", "e2e/checkout.spec.ts"))
	r, _ := status(t, fx.root, "--no-store", "--receipt", named)
	got := row(t, r, "checkout", "checkout.basic")
	if len(got) != 1 || got[0].TestKey != checkoutKey || got[0].Result != "failed" || got[0].Granularity != "file" || got[0].Stale ||
		got[0].SourceCommit != fx.head || got[0].Authority != appflows.AuthorityLocallyObserved {
		t.Fatalf("checkout from the receipt: %+v", got)
	}
	if other := row(t, r, "search", "search.basic"); other[0].Result != appflows.ResultNotObserved {
		t.Fatalf("search is not in the receipt: %+v", other)
	}
	if last := r.ResultSources[len(r.ResultSources)-1]; last != (appflows.ResultSource{Kind: appflows.SourceReceipt, Path: named, State: appflows.SourceRead, Records: 1}) {
		t.Fatalf("receipt source %+v", last)
	}
	gitDir := git(t, fx.root, "rev-parse", "--absolute-git-dir")
	write(t, filepath.Join(gitDir, "corvint"), map[string]string{"test-run.json": string(receipt(t, fx.head, fx.tree, "PASS", "e2e/search.spec.ts"))})
	r, _ = status(t, fx.root)
	if got := row(t, r, "search", "search.basic"); got[0].Result != "passed" || got[0].Granularity != "file" {
		t.Fatalf("search from the default receipt: %+v", got)
	}
	if r.ResultSources[1] != (appflows.ResultSource{Kind: appflows.SourceReceipt, Path: appflows.DefaultReceiptLabel, State: appflows.SourceRead, Records: 1}) {
		t.Fatalf("default receipt source %+v", r.ResultSources)
	}
}

// AFL-V1-002: an ingested Playwright result names the title itself (granularity test), and a
// result recorded at another commit is stale with its source commit.
func TestAFLV1002IngestedAndStaleResults_V15623(t *testing.T) {
	fx := newShop(t)
	baseTree := git(t, fx.root, "rev-parse", fx.b+"^{tree}")
	evidence := file(t, "runs.jsonl", append(ingested(t, checkoutKey, fx.head, fx.tree, "passed"), ingested(t, searchKey, fx.b, baseTree, "failed")...))
	r, _ := status(t, fx.root, "--no-store", "--evidence", evidence)
	if got := row(t, r, "checkout", "checkout.basic"); got[0].Result != "passed" || got[0].Granularity != "test" || got[0].Stale || got[0].Authority != appflows.AuthorityIngested {
		t.Fatalf("ingested: %+v", got)
	}
	if got := row(t, r, "search", "search.basic"); got[0].Result != "failed" || !got[0].Stale || got[0].SourceCommit != fx.b || got[0].SourceTree != baseTree {
		t.Fatalf("stale: %+v", got)
	}
	// Named receipts are read before named evidence files, so the evidence record is the newest.
	named := file(t, "receipt.json", receipt(t, fx.head, fx.tree, "PASS", "e2e/search.spec.ts"))
	r, _ = status(t, fx.root, "--no-store", "--evidence", evidence, "--receipt", named)
	if got := row(t, r, "search", "search.basic"); got[0].Result != "failed" || !got[0].Stale {
		t.Fatalf("named --evidence files come after --receipt files: %+v", got)
	}
}

// AFL-V1-006: a malformed, oversized or other-profile receipt or evidence file, or an unknown
// flow-file profile, is one typed refusal with exit 2 and no partial report.
func TestAFLV1006InvalidInputRefusedWhole_V15623(t *testing.T) {
	fx := newShop(t)
	oversized := bytes.Repeat([]byte(" "), appflows.MaxBytes+1)
	for name, args := range map[string][]string{
		"malformed receipt":     {"--receipt", file(t, "bad.json", []byte("{not json"))},
		"other profile receipt": {"--receipt", file(t, "other.json", []byte(`{"profile":"corvint-affected/1"}`))},
		"ingested as receipt":   {"--receipt", file(t, "runs.jsonl", ingested(t, checkoutKey, fx.head, fx.tree, "passed"))},
		"oversized receipt":     {"--receipt", file(t, "big.json", oversized)},
		"missing receipt":       {"--receipt", filepath.Join(t.TempDir(), "absent.json")},
		"malformed evidence":    {"--evidence", file(t, "bad.jsonl", []byte("{}\n"))},
		"unknown flag":          {"--registry", "runs/registry.json"},
	} {
		code, out, diagnostic := report(t, fx.root, "status", append([]string{"--flows", "flows", "--no-store"}, args...)...)
		if code != 2 || out != "" || !strings.Contains(diagnostic, `"code":"invalid-arguments"`) {
			t.Errorf("%s: %d %q %s", name, code, out, diagnostic)
		}
	}
	write(t, fx.root, map[string]string{appflows.FlowsDeclarationPath: `{"profile":"corvint-flows/9","flows":"flows"}` + "\n"})
	commit(t, fx.root, "D: a newer flow file")
	for _, verb := range [][]string{{"status", "--flows", "flows"}, {"impact", "--flows", "flows", "--base", fx.b}, {"gaps", "--flows", "flows"}} {
		code, out, diagnostic := report(t, fx.root, verb[0], verb[1:]...)
		if code != 2 || out != "" || !strings.Contains(diagnostic, "FLOWS_SCHEMA_UNSUPPORTED") || !strings.Contains(diagnostic, "9") {
			t.Errorf("flows %s on corvint-flows/9: %d %q %s", verb[0], code, out, diagnostic)
		}
	}
}

// AFL-V1-005: two runs on the same inputs print the same bytes and write nothing.
func TestAFLV1005RepeatedRunIsByteIdenticalAndReadOnly_V15623(t *testing.T) {
	fx := newShop(t)
	named := file(t, "test-run.json", receipt(t, fx.head, fx.tree, "PASS", "e2e/checkout.spec.ts"))
	before := git(t, fx.root, "status", "--porcelain", "--ignored")
	gitDir := git(t, fx.root, "rev-parse", "--absolute-git-dir")
	if _, err := os.Stat(filepath.Join(gitDir, "corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fixture already has %s/corvint", gitDir)
	}
	for _, args := range [][]string{{"--receipt", named}, {"--base", fx.b}} {
		_, first := status(t, fx.root, args...)
		_, second := status(t, fx.root, args...)
		if first != second {
			t.Fatalf("%v differs:\n%s\n%s", args, first, second)
		}
	}
	if after := git(t, fx.root, "status", "--porcelain", "--ignored"); after != before {
		t.Fatalf("status wrote the worktree: %q -> %q", before, after)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status wrote %s/corvint: %v", gitDir, err)
	}
}

// AFL-V1-004, APK-V0-007 comparison: with --base the report holds exactly the flows and
// variations flows impact names for the same base and head.
func TestAFLV1004BaseMatchesFlowsImpact_V15623(t *testing.T) {
	fx := newShop(t)
	code, out, diagnostic := report(t, fx.root, "impact", "--flows", "flows", "--base", fx.b)
	if code != 0 {
		t.Fatalf("impact %d %s", code, diagnostic)
	}
	var impact appflows.ImpactReport
	if err := json.Unmarshal([]byte(out), &impact); err != nil {
		t.Fatal(err)
	}
	var impacted []string
	for _, f := range impact.Flows {
		for _, v := range f.Variations {
			impacted = append(impacted, f.FlowID+"/"+v)
		}
	}
	r, _ := status(t, fx.root, "--base", fx.b)
	var reported []string
	for _, f := range r.Flows {
		for _, v := range f.Variations {
			reported = append(reported, f.FlowID+"/"+v.VariationID)
		}
	}
	if !slices.Equal(reported, impacted) || !slices.Equal(reported, []string{"checkout/checkout.basic"}) {
		t.Fatalf("status --base %v, flows impact %v; want both [checkout/checkout.basic]", reported, impacted)
	}
	if !r.LimitedByBase || r.Base != fx.b || r.ImpactGraph == nil || r.ImpactGraph.Digest != impact.Graph.Digest || r.ImpactGraph.Scope != impact.Graph.Scope {
		t.Fatalf("status --base carries base %q limited %v graph %+v, impact graph %+v", r.Base, r.LimitedByBase, r.ImpactGraph, impact.Graph)
	}
	if code, out, diagnostic := report(t, fx.root, "status", "--flows", "flows", "--base", strings.Repeat("0", 40)); code != 2 || out != "" || !strings.Contains(diagnostic, "unsupported-affected-revision") {
		t.Fatalf("unknown base: %d %q %s", code, out, diagnostic)
	}
}

// AFL-V1-003, AFL-V1-007: corvint.flows_status and corvint.flows_impact on the flows MCP profile
// return the command's report body: the same canonical bytes for the same inputs.
func TestAFLV1003V1007ToolsReturnTheCommandReports_V15623(t *testing.T) {
	fx := newShop(t)
	write(t, fx.root, map[string]string{"runs/test-run.json": string(receipt(t, fx.head, fx.tree, "PASS", "e2e/checkout.spec.ts"))})
	registry, err := bridge.NewFlows(fx.root)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, tool := range registry.Tools() {
		listed[tool.Name] = true
	}
	if !listed[bridge.ToolFlowsStatus] || !listed[bridge.ToolFlowsImpact] {
		t.Fatalf("the flows profile does not list %s and %s: %v", bridge.ToolFlowsStatus, bridge.ToolFlowsImpact, listed)
	}
	// The command runs from the root, so the relative receipt path names the same file.
	t.Chdir(fx.root)
	for _, test := range []struct {
		tool      string
		arguments map[string]any
		verb      string
		args      []string
	}{
		{bridge.ToolFlowsStatus, map[string]any{"flows": "flows"}, "status", []string{"--flows", "flows"}},
		{bridge.ToolFlowsStatus, map[string]any{"flows": "flows", "receipts": []any{"runs/test-run.json"}, "base": fx.b}, "status", []string{"--flows", "flows", "--receipt", "runs/test-run.json", "--base", fx.b}},
		{bridge.ToolFlowsImpact, map[string]any{"flows": "flows", "base": fx.b}, "impact", []string{"--flows", "flows", "--base", fx.b}},
	} {
		code, out, diagnostic := report(t, fx.root, test.verb, test.args...)
		if code != 0 {
			t.Fatalf("flows %s %v: %d %s", test.verb, test.args, code, diagnostic)
		}
		raw, err := json.Marshal(test.arguments)
		if err != nil {
			t.Fatal(err)
		}
		result, callErr := registry.Call(context.Background(), test.tool, raw)
		if callErr != nil || result.State != "READY" {
			t.Fatalf("%s %s: %#v %#v", test.tool, raw, callErr, result.Abstention)
		}
		tool, err := json.Marshal(result.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := canonical(t, tool), canonical(t, []byte(out)); !bytes.Equal(got, want) {
			t.Errorf("%s %s differs from flows %s:\n%s\n%s", test.tool, raw, test.verb, got, want)
		}
	}
}

// canonical re-encodes a JSON body with sorted members and exact numbers, so two encoders of the
// same report compare byte for byte.
func canonical(t *testing.T, raw []byte) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
