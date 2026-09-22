package contextindex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPacketBudgetMatchesFrozenPythonOracleVector(t *testing.T) {
	input := readJSONMap(t, filepath.Join("testdata", "oracle-budgeted-omission-input.json"))
	wrapper := readJSONMap(t, filepath.Join(
		"..", "..", "conformance", "go-query-start-v0", "pending-capability", "oracle-budgeted-omission.json",
	))
	expected := wrapper["context"].(map[string]any)
	budget := 1500
	got, err := compileReceipt(input, &budget, &Index{Sources: map[string]Source{}})
	if err != nil {
		t.Fatal(err)
	}
	gotBytes, err := CanonicalJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	expectedBytes, err := CanonicalJSON(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, expectedBytes) {
		t.Fatalf("budgeted context differs from frozen Python oracle\ngot coverage=%v\nwant coverage=%v", got["coverage"], expected["coverage"])
	}
}

func TestCoverageComputesAdvisoryAndCriticalMissing(t *testing.T) {
	receipt := map[string]any{
		"mode": "impact",
		"results": []any{
			map[string]any{"kind": "learned-path", "id": "advisory.go", "evidence": []any{}},
		},
	}
	critical := []any{"path:required.go"}
	if err := setCoverage(receipt, 3, critical, nil, nil); err != nil {
		t.Fatal(err)
	}
	coverage := receipt["coverage"].(map[string]any)
	if coverage["requested_results"] != 3 || coverage["included_results"] != 1 ||
		coverage["omitted_results"] != 2 || coverage["authoritative_results"] != 0 ||
		coverage["advisory_results"] != 1 {
		t.Fatalf("coverage counts=%v", coverage)
	}
	if missing := anySlice(coverage["critical_missing"]); len(missing) != 1 || missing[0] != "path:required.go" {
		t.Fatalf("critical_missing=%v", missing)
	}
	wantUncertainty := []any{
		"learned path relationships are advisory",
		"2 ranked results omitted by result limit",
	}
	if got := anySlice(coverage["uncertainty"]); !equalAnyStrings(got, wantUncertainty) {
		t.Fatalf("uncertainty=%v want=%v", got, wantUncertainty)
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if coverage["packet_bytes"] != len(encoded) || coverage["within_budget"] != true {
		t.Fatalf("packet coverage=%v encoded=%d", coverage, len(encoded))
	}
}

func TestPacketBudgetCompactsCriticalLists(t *testing.T) {
	receipt := budgetTestReceipt("impact", map[string]any{"paths": []any{"dirty.go"}, "limit": 10})
	results := make([]any, 10)
	for index := range results {
		results[index] = map[string]any{
			"kind": "path", "id": strings.Repeat("x", 180) + string(rune('a'+index)), "evidence": []any{},
		}
	}
	receipt["results"] = results
	budget := MinPacketBytes
	packet, err := compileReceipt(receipt, &budget, &Index{Sources: map[string]Source{}})
	if err != nil {
		t.Fatal(err)
	}
	coverage := packet["coverage"].(map[string]any)
	if packet["state"] != "CRITICAL_EVIDENCE_OVERFLOW" || coverage["critical_count"] != 10 ||
		coverage["critical_missing_count"] != 10 || len(anySlice(coverage["critical"])) != 0 ||
		len(anySlice(coverage["critical_missing"])) != 0 {
		t.Fatalf("state=%v coverage=%v", packet["state"], coverage)
	}
	for _, field := range []string{"critical_sha256", "critical_missing_sha256"} {
		if len(stringValue(coverage[field])) != 64 {
			t.Fatalf("%s=%v", field, coverage[field])
		}
	}
}

func TestPacketBudgetCompactsOversizedRequest(t *testing.T) {
	receipt := budgetTestReceipt("query", map[string]any{
		"text": strings.Repeat("z", 800), "limit": 10,
	})
	budget := MinPacketBytes
	packet, err := compileReceipt(receipt, &budget, &Index{Sources: map[string]Source{}})
	if err != nil {
		t.Fatal(err)
	}
	request := packet["request"].(map[string]any)
	if request["budget_bytes"] != MinPacketBytes || request["omitted_by_budget"] != 2 ||
		len(stringValue(request["request_sha256"])) != 64 {
		t.Fatalf("request=%v", request)
	}
}

func budgetTestReceipt(mode string, request map[string]any) map[string]any {
	return map[string]any{
		"schema_version": 1, "mode": mode, "request": request, "state": "READY", "revision": "revision",
		"freshness": map[string]any{
			"state": "fresh", "scope": "git", "revision": "revision",
			"mixed_paths": []any{}, "mixed_path_count": 0,
		},
		"results": []any{}, "exclusions": map[string]any{"count": 0, "samples": []any{}},
		"verification": []any{},
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return normalizeJSONNumbers(result).(map[string]any)
}

func normalizeJSONNumbers(value any) any {
	switch item := value.(type) {
	case float64:
		return int(item)
	case map[string]any:
		for key, child := range item {
			item[key] = normalizeJSONNumbers(child)
		}
	case []any:
		for index, child := range item {
			item[index] = normalizeJSONNumbers(child)
		}
	}
	return value
}

func equalAnyStrings(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// budgetSweepPath holds the frozen Python-oracle compilation of
// frozenBudgetInputPath at every budget where the oracle's selection changes.
// Both files are oracle output: no expectation in this file is produced by the
// Go packet compiler it certifies.
const (
	frozenBudgetInputPath = "testdata/oracle-budgeted-omission-input.json"
	budgetSweepPath       = "../../conformance/go-query-start-v0/pending-capability/oracle-budget-boundary-sweep.json"
)

// TestPacketBudgetBoundariesMatchFrozenPythonOracleSweep walks the overflow
// boundary one byte at a time. Each admitted budget is paired with the byte
// below it, so a selector that rounded the wrong way — or stopped at the first
// result too large to fit instead of continuing to a smaller one — fails here
// rather than only on an unrelated corpus.
func TestPacketBudgetBoundariesMatchFrozenPythonOracleSweep(t *testing.T) {
	sweep := readJSONMap(t, budgetSweepPath)
	for _, raw := range anySlice(sweep["vectors"]) {
		vector := raw.(map[string]any)
		budget := vector["budget_bytes"].(int)
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			packet, err := compileReceipt(readJSONMap(t, frozenBudgetInputPath), &budget, captureProjectIndex(t, sweep))
			if err != nil {
				t.Fatal(err)
			}
			assertCanonicalEqual(t, packet, vector["context"].(map[string]any))
		})
	}
}

// TestPacketBudgetRefusesBelowTheEnvelopeFloor pins the other end of the
// boundary: a budget that cannot hold the envelope even with zero results, a
// digested request, and compacted coverage lists must refuse in the oracle's
// words instead of emitting an over-budget packet.
func TestPacketBudgetRefusesBelowTheEnvelopeFloor(t *testing.T) {
	sweep := readJSONMap(t, budgetSweepPath)
	floor := sweep["envelope_floor"].(map[string]any)
	refused := floor["refused_budget_bytes"].(int)
	packet, err := compileReceipt(inflatedBudgetInput(t, floor), &refused, captureProjectIndex(t, sweep))
	if err == nil {
		t.Fatalf("budget_bytes=%d admitted a packet: %v", refused, packet)
	}
	if err.Error() != stringValue(floor["error"]) {
		t.Fatalf("refusal=%q want=%q", err.Error(), floor["error"])
	}

	admitted := floor["admitted_budget_bytes"].(int)
	packet, err = compileReceipt(inflatedBudgetInput(t, floor), &admitted, captureProjectIndex(t, sweep))
	if err != nil {
		t.Fatal(err)
	}
	assertCanonicalEqual(t, packet, floor["context"].(map[string]any))
}

// inflatedBudgetInput rebuilds the receipt the oracle refused: the frozen input
// with an opaque revision long enough that the compacted envelope alone
// overruns the budget floor.
func inflatedBudgetInput(t *testing.T, floor map[string]any) map[string]any {
	t.Helper()
	input := readJSONMap(t, frozenBudgetInputPath)
	input["revision"] = stringValue(floor["input_revision_override"])
	return input
}

// captureProjectIndex rebuilds the source inventory the oracle compiled the
// sweep against. It is behaviourally complete for this input: the capture
// revision's tree has no record ledgers and no root package.json, so those two
// absences and the Go/Python toolchain files are the whole of what the
// verification plan reads.
func captureProjectIndex(t *testing.T, sweep map[string]any) *Index {
	t.Helper()
	sources := make(map[string]Source)
	for _, raw := range anySlice(sweep["projectFiles"]) {
		sources[stringValue(raw)] = Source{}
	}
	if len(sources) == 0 {
		t.Fatal("sweep declares no project files")
	}
	return &Index{Sources: sources}
}

func assertCanonicalEqual(t *testing.T, got, want map[string]any) {
	t.Helper()
	gotBytes, err := CanonicalJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := CanonicalJSON(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("packet differs from frozen Python oracle\ngot  %s\nwant %s", gotBytes, wantBytes)
	}
}

// TestPacketBudgetWithAbsentInventoryMatchesFrozenPythonOracle covers the
// caller that supplies no source inventory at all. The oracle reads that as an
// unknown project rather than a signal-free one
// (src/context_corvint_learning.py:196 `not project_files or ...`), so the gate
// it closes with — and therefore the packet size the budget is measured
// against — differs from the signal-free fallback.
func TestPacketBudgetWithAbsentInventoryMatchesFrozenPythonOracle(t *testing.T) {
	vector := readJSONMap(t, budgetSweepPath)["absent_inventory"].(map[string]any)
	budget := vector["budget_bytes"].(int)
	packet, err := compileReceipt(readJSONMap(t, frozenBudgetInputPath), &budget, &Index{Sources: map[string]Source{}})
	if err != nil {
		t.Fatal(err)
	}
	assertCanonicalEqual(t, packet, vector["context"].(map[string]any))
}

// TestPacketBudgetKeepsTheAbstentionReason pins the observability half of the
// contract: a packet the relevance floor withdrew must still name what
// withdrew it after budget compaction, including under the repository intent
// that used to drop the abstention outright. An abstention that cannot say why
// is indistinguishable from an empty result set.
func TestPacketBudgetKeepsTheAbstentionReason(t *testing.T) {
	for _, testCase := range []struct {
		name, intent, reason string
	}{
		{name: "repository intent", intent: "repository", reason: "below-relevance-floor"},
		{name: "named intent", intent: "authentication", reason: "no-relevant-candidates"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			receipt := budgetTestReceipt("query", map[string]any{"text": "sourdough", "limit": 8})
			receipt["state"] = "OUT_OF_SCOPE"
			receipt["intent"] = map[string]any{
				"id": testCase.intent, "confidence": "low", "matched_terms": []any{},
			}
			receipt["abstention"] = map[string]any{"active": true, "reason": testCase.reason}
			budget := 4096
			packet, err := compileReceipt(receipt, &budget, &Index{Sources: map[string]Source{"go.mod": {}}})
			if err != nil {
				t.Fatal(err)
			}
			abstention, ok := packet["abstention"].(map[string]any)
			if !ok {
				t.Fatalf("budgeted packet dropped its abstention: %v", packet)
			}
			if abstention["active"] != true || abstention["reason"] != testCase.reason {
				t.Fatalf("abstention=%v want active with reason %q", abstention, testCase.reason)
			}
		})
	}
}

// envelopeFloorWorstCaseReceipt builds the largest mandatory abstaining query
// packet the default (non-NEEDS_WIDENING) branch of compactBudgetEnvelope can
// produce: mixed-worktree freshness with a mixed_paths_sha256, the longest
// abstention.reason literal the default branch emits
// (`unindexed-worktree-changes`, from eval_query.go), a fully populated
// learning block whose local_trace_state is its longest value
// (`blocked-mixed-worktree`, from authorityTraceState in history.go -- itself
// mandatory whenever the worktree is mixed), the longest compacted intent
// shape (`project-operations`, the longer of the two non-repository
// classifier ids in query.go, with the fixed `confidence: "high"` that always
// accompanies a non-repository id), and a 64-hex-character revision (the
// sha256 Git object-format length from validObjectID in git.go, longer than
// the 40-character sha1 length). MinPacketBytes must be re-derived from this
// construction whenever any of those mandatory fields or their longest
// literal changes.
func envelopeFloorWorstCaseReceipt() (map[string]any, *Index) {
	sha64 := strings.Repeat("a", 64)
	receipt := map[string]any{
		"schema_version": 1, "mode": "query",
		"request":  map[string]any{"text": "sourdough starter troubleshooting", "limit": 8},
		"state":    "OUT_OF_SCOPE",
		"revision": sha64,
		"freshness": map[string]any{
			"state": "mixed-worktree", "scope": "git", "revision": sha64,
			"mixed_paths": []any{"a.go"}, "mixed_path_count": 1,
		},
		"results":      []any{},
		"exclusions":   map[string]any{"count": 0, "samples": []any{}},
		"verification": []any{},
		"learning": map[string]any{
			"history_tip": sha64, "history_digest": sha64,
			"local_trace_state": "blocked-mixed-worktree", "local_trace_count": 0,
			"matched_local_traces": 0, "advisory_candidates": 0,
		},
		"intent": map[string]any{
			"id": "project-operations", "confidence": "high",
			"matched_terms": []any{"queue", "backlog"},
		},
		"abstention": map[string]any{"active": true, "reason": "unindexed-worktree-changes"},
	}
	index := &Index{Sources: map[string]Source{"go.mod": {}}, DirtyPaths: []string{"a.go"}}
	return receipt, index
}

// TestMinPacketBytesIsTheWorstCaseAbstainingEnvelopeRoundedUp re-derives the
// packet-budget floor from first principles instead of pinning the one 1077
// byte case GPK-V0-045 happened to observe: it finds the smallest budget the
// worst-case mandatory abstaining envelope can fit in, and asserts
// MinPacketBytes is that value rounded up to the next 64-byte boundary. A
// change to the mandatory field set (see envelopeFloorWorstCaseReceipt) that
// grows the envelope must fail this test until MinPacketBytes is raised to
// match.
func TestMinPacketBytesIsTheWorstCaseAbstainingEnvelopeRoundedUp(t *testing.T) {
	receipt, index := envelopeFloorWorstCaseReceipt()
	minimumAdmitted := -1
	for budget := 512; budget <= 2048; budget++ {
		b := budget
		if _, err := compileReceipt(cloneMap(receipt), &b, index); err == nil {
			minimumAdmitted = budget
			break
		}
	}
	if minimumAdmitted < 0 {
		t.Fatal("no budget in [512, 2048] admitted the worst-case abstaining envelope")
	}
	wantFloor := ((minimumAdmitted + 63) / 64) * 64
	if MinPacketBytes != wantFloor {
		t.Fatalf("MinPacketBytes=%d, want %d (worst-case envelope needs %d bytes, rounded up to 64)",
			MinPacketBytes, wantFloor, minimumAdmitted)
	}
	b := MinPacketBytes
	if _, err := compileReceipt(cloneMap(receipt), &b, index); err != nil {
		t.Fatalf("MinPacketBytes=%d still refuses the worst-case abstaining envelope: %v", MinPacketBytes, err)
	}
}

// TestCoverageWithdrawsAuthorityFromASyntaxOnlyQueryPacket pins the honesty
// half: a query answered only by symbols whose names happen to match the task
// carries no project-owned corroboration, so it names that in `uncertainty`
// and counts no result as authoritative. One project-owned citation anywhere
// in the packet restores the ordinary count.
func TestCoverageWithdrawsAuthorityFromASyntaxOnlyQueryPacket(t *testing.T) {
	syntax := func(id string) any {
		return map[string]any{"kind": "symbol", "id": id, "evidence": []any{
			map[string]any{"path": "a.go", "line": 1, "authority": SyntaxAuthority, "reason": "declares " + id},
		}}
	}
	instruction := map[string]any{"kind": "instructions", "id": "AGENTS.md", "evidence": []any{
		map[string]any{"path": "AGENTS.md", "line": 1, "authority": "project-instructions", "reason": "matches"},
	}}
	for _, testCase := range []struct {
		name, mode    string
		results       []any
		authoritative int
		named         bool
	}{
		{name: "syntax only", mode: "query", results: []any{syntax("a"), syntax("b")}, authoritative: 0, named: true},
		{name: "corroborated", mode: "query", results: []any{syntax("a"), instruction}, authoritative: 2},
		{name: "other mode", mode: "impact", results: []any{syntax("a")}, authoritative: 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			receipt := budgetTestReceipt(testCase.mode, map[string]any{"text": "task", "limit": 8})
			receipt["results"] = testCase.results
			if err := setCoverage(receipt, len(testCase.results), []any{}, nil, nil); err != nil {
				t.Fatal(err)
			}
			coverage := receipt["coverage"].(map[string]any)
			if coverage["authoritative_results"] != testCase.authoritative {
				t.Fatalf("authoritative_results=%v want %d", coverage["authoritative_results"], testCase.authoritative)
			}
			named := false
			for _, entry := range anySlice(coverage["uncertainty"]) {
				named = named || entry == SyntaxOnlyUncertainty
			}
			if named != testCase.named {
				t.Fatalf("uncertainty=%v want the syntax-only line: %v", coverage["uncertainty"], testCase.named)
			}
		})
	}
}

// `unparsed` and `extraction` are Go-only envelope members carrying the same
// {count, samples} shape as `exclusions`, and nothing in the oracle's envelope
// forced the question of compacting them. Their samples were charged against
// the evidence budget: on this repository's file-change receipt the 925-byte
// `unparsed` list was the whole of a 901-byte envelope excess over the oracle,
// and the greedy selector then skipped a `repository-spec` document scoring
// 825 because it no longer fit, taking a smaller `syntax` result scoring 775
// instead. The disclosure must survive as a count; only its samples go.
func TestBudgetedEnvelopeCompactsTheGoOnlyDisclosureMembers(t *testing.T) {
	receipt := map[string]any{
		"schema_version": 1, "mode": "impact",
		"request":   map[string]any{"paths": []any{"a.go"}, "limit": 10},
		"state":     "READY",
		"revision":  strings.Repeat("a", 40),
		"freshness": map[string]any{"state": "clean", "scope": "git", "mixed_path_count": 0},
		"exclusions": map[string]any{
			"count": 2, "samples": []any{map[string]any{"path": "x", "reason": "y"}},
		},
		"unparsed": map[string]any{
			"count": 7, "samples": []any{
				map[string]any{"path": "one.py", "reason": UnparsedPythonGrammar},
				map[string]any{"path": "two.go", "reason": UnparsedGoGrammar},
			},
		},
		"extraction": map[string]any{
			"count": 3, "samples": []any{map[string]any{"path": "three.ts", "reason": "lexical"}},
		},
		"results":      []any{},
		"verification": []any{},
	}
	if err := compactBudgetEnvelope(receipt); err != nil {
		t.Fatal(err)
	}
	for name, wantCount := range map[string]int{"unparsed": 7, "extraction": 3} {
		member, ok := receipt[name].(map[string]any)
		if !ok {
			t.Fatalf("%s member was dropped entirely: %v", name, receipt[name])
		}
		if member["count"] != wantCount {
			t.Fatalf("%s count = %v, want %d: the disclosure must survive the budget", name, member["count"], wantCount)
		}
		if _, present := member["samples"]; present {
			t.Fatalf("%s kept its samples under a budget, charging the evidence budget for them: %v", name, member)
		}
		if member["samples_omitted_by_budget"] == nil {
			t.Fatalf("%s omitted its samples without saying so: %v", name, member)
		}
	}
}
