package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/appflows"
)

// e2eTestsPerFlow is how many generated tests one generated flow links; it stays below the
// per-flow variation and link bounds.
const e2eTestsPerFlow = 256

// scaleE2ECorpus adds generated flows, one spec file per flow and e2eTestsPerFlow inventoried tests
// per spec to the frozen corpus, so the inventory holds tests generated tests plus the corpus five.
// Every generated link is reviewed at A, so a change no generated flow reaches omits every
// generated test on the reviewed-links basis.
func scaleE2ECorpus(t *testing.T, tests int) e2eCorpus {
	t.Helper()
	corpus := loadE2ECorpus(t)
	var provider map[string]any
	if err := json.Unmarshal([]byte(corpus.Files["e2e-provider.json"]), &provider); err != nil {
		t.Fatal(err)
	}
	inventory := provider["inventory"].([]any)
	for flow := 0; flow*e2eTestsPerFlow < tests; flow++ {
		id := fmt.Sprintf("g%03d", flow)
		links := []string{fmt.Sprintf(`{"basis": "declared", "from": "step", "reviewed_at": "@ANCHOR@", "target": {"path": "src/gen/%s.ts", "type": "source"}}`, id)}
		for k := 0; k < e2eTestsPerFlow && flow*e2eTestsPerFlow+k < tests; k++ {
			key := fmt.Sprintf("%s.spec.ts > k%03d", id, k)
			links = append(links, fmt.Sprintf(`{"basis": "declared", "from": "%s.v", "reviewed_at": "@ANCHOR@", "target": {"path": "e2e/%s.spec.ts", "test_key": %q, "type": "test"}}`, id, id, key))
			inventory = append(inventory, map[string]any{"path": "e2e/" + id + ".spec.ts", "project": "chromium", "test_key": key})
		}
		corpus.Files["flows/"+id+".json"] = fmt.Sprintf(`{"actor": "shopper", "flow_id": %q, "kind": "ui", "links": [%s], "outcomes": [{"behavior": "done visible", "locator": "#done", "matcher": "toBeVisible", "outcome_id": "done", "value": "done"}], "preconditions": [], "revision": 1, "schema": "application-flow-intent/1", "steps": [{"action": "do step", "step_id": "step"}], "variations": [{"observable_facts": [], "outcomes": ["done"], "preconditions": [], "projects": ["chromium"], "steps": ["step"], "variation_id": "%s.v"}]}`,
			id, strings.Join(links, ", "), id)
		corpus.Files["e2e/"+id+".spec.ts"] = "test(\"generated\", () => 0);\n"
		corpus.Files["src/gen/"+id+".ts"] = "export const " + id + " = 0;\n"
	}
	provider["inventory"] = inventory
	raw, err := json.Marshal(provider)
	if err != nil {
		t.Fatal(err)
	}
	corpus.Files["e2e-provider.json"] = string(raw)
	return corpus
}

// AFU-V1-065: a provider inventory well above the former 1,024-test bound plans without
// e2e-bound-exceeded and narrows; each reviewed-links proof is stated once per flow (AFU-V1-066).
func TestAFUV1065LargeInventoryNarrows(t *testing.T) {
	t.Parallel()
	generated := 3 * 1024
	corpus := scaleE2ECorpus(t, generated)
	selection, _, _ := runE2ECase(t, corpus, corpus.find(t, "search-source"))
	if selection.State != "narrow-selection-allowed" || slices.Contains(e2eCodes(selection), appflows.CodeBoundExceeded) {
		t.Fatalf("large inventory did not narrow: state %s codes %v", selection.State, e2eCodes(selection))
	}
	if got := selection.OmissionsPerBasis[appflows.BasisReviewedLinks]; got != generated+1 {
		t.Fatalf("reviewed-links omissions %d, want %d", got, generated+1)
	}
	flows := (generated + e2eTestsPerFlow - 1) / e2eTestsPerFlow
	if len(selection.Proofs) != 3+1+flows || !slices.Equal(selection.DisjointFrom, []string{"src/search.ts"}) {
		t.Fatalf("proofs %d (want %d), disjoint_from %v", len(selection.Proofs), 3+1+flows, selection.DisjointFrom)
	}
	for _, o := range selection.OmittedTests {
		e2eProof(t, selection, o)
	}
}

// AFU-V1-065: one test above the documented inventory ceiling keeps the closed e2e-bound-exceeded
// full-suite fallback.
func TestAFUV1065InventoryAboveCeilingRefused(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	var provider map[string]any
	if err := json.Unmarshal([]byte(corpus.Files["e2e-provider.json"]), &provider); err != nil {
		t.Fatal(err)
	}
	inventory := provider["inventory"].([]any)
	for i := len(inventory); i <= appflows.MaxSelectionTests; i++ {
		inventory = append(inventory, map[string]any{"path": "e2e/cart.spec.ts", "project": "chromium", "test_key": fmt.Sprintf("cart.spec.ts > extra %05d", i)})
	}
	provider["inventory"] = inventory
	raw, err := json.Marshal(provider)
	if err != nil {
		t.Fatal(err)
	}
	corpus.Files["e2e-provider.json"] = string(raw)
	selection, _, _ := runE2ECase(t, corpus, corpus.find(t, "search-source"))
	e2eFullSuite(t, "inventory-above-ceiling", selection)
	if want := (appflows.E2EFallback{Code: appflows.CodeBoundExceeded, Subject: "inventory"}); !slices.Equal(selection.Fallback, []appflows.E2EFallback{want}) {
		t.Fatalf("fallback %+v, want only %+v", selection.Fallback, want)
	}
	if len(selection.Selected) != appflows.MaxSelectionTests+1 || len(selection.Proofs) != 0 {
		t.Fatalf("full suite %d tests, proofs %d", len(selection.Selected), len(selection.Proofs))
	}
}

// AFU-V1-065 measurement: CORVINT_E2E_SCALE_TESTS=16384 plans at the documented inventory ceiling
// and logs elapsed time, allocation and receipt size. It is skipped by default.
func TestAFUV1065CeilingMeasurement(t *testing.T) {
	tests, err := strconv.Atoi(os.Getenv("CORVINT_E2E_SCALE_TESTS"))
	if err != nil || tests <= 0 {
		t.Skip("set CORVINT_E2E_SCALE_TESTS to measure")
	}
	corpus := scaleE2ECorpus(t, tests-5)
	root, base, _ := buildE2ECase(t, corpus, corpus.find(t, "search-source"))
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	receipt, stdout, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", "e2e-provider.json", "--selection-profile", "e2e-safe")
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	if code != 0 {
		t.Fatalf("affected exited %d: %s", code, stderr)
	}
	selection := testSelection(t, receipt)
	t.Logf("inventory=%d state=%v omitted=%d proofs=%d elapsed=%s receipt_bytes=%d total_alloc_bytes=%d heap_sys_bytes=%d",
		tests, selection["state"], len(selection["omitted_tests"].([]any)), len(selection["proofs"].([]any)), elapsed, len(stdout), after.TotalAlloc-before.TotalAlloc, after.HeapSys)
	if selection["state"] != "narrow-selection-allowed" {
		t.Fatalf("ceiling inventory did not narrow: %v", selection["fallback"])
	}
}
