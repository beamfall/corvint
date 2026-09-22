package contextindex

import (
	"context"
	"strings"
	"testing"
)

// evalQueryCoverageRepository carries three canonical records one task selects,
// so the ranking ceiling has something to drop at a small limit, plus a test
// declaration no query can reach through the symbol ranker.
func evalQueryCoverageRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"go.mod":     "module example.test/coverage\n\ngo 1.27.0\n",
		"testing/features.yaml": "features:\n" +
			"  - id: session-revocation-device\n    area: auth\n    summary: Revoke a device session.\n    adr: []\n    applies: [server]\n    status: shipped\n" +
			"  - id: session-revocation-token\n    area: auth\n    summary: Revoke a token session.\n    adr: []\n    applies: [server]\n    status: shipped\n" +
			"  - id: session-revocation-audit\n    area: auth\n    summary: Audit a revoked session.\n    adr: []\n    applies: [server]\n    status: shipped\n",
		"testing/scenarios.yaml": "scenarios:\n" +
			"  - id: session-revocation-denied\n    area: auth\n    summary: A revoked session is denied.\n    features: [session-revocation-device]\n    applies: [server]\n    status: shipped\n",
		"internal/auth/session.go":      "package auth\n\nfunc ComputeSessionDigest() string { return \"\" }\n",
		"internal/auth/session_test.go": "package auth\n\nfunc TestSomethingSpecific() {}\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "revoke device and token sessions")
	return root
}

func evalQueryCoverage(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	coverage, ok := receipt["coverage"].(map[string]any)
	if !ok {
		t.Fatalf("receipt carries no coverage block: %v", receipt)
	}
	return coverage
}

func evalQueryCoverageInt(t *testing.T, coverage map[string]any, field string) int {
	t.Helper()
	value, ok := coverage[field].(int)
	if !ok {
		t.Fatalf("coverage.%s is %T, want int", field, coverage[field])
	}
	return value
}

func evalQueryUncertainty(t *testing.T, coverage map[string]any) []string {
	t.Helper()
	lines := make([]string, 0)
	for _, raw := range anySlice(coverage["uncertainty"]) {
		lines = append(lines, stringValue(raw))
	}
	return lines
}

// TestQueryCoverageCountsAdmittedCandidatesPastLimit pins GPK-V0-040 on the
// query verb: the coverage block is denominated in the universe the ranking
// admitted, so a packet cut down by `--limit` says how much it is not showing
// instead of reporting a completeness it never measured. The selection itself
// is unchanged, which is why the limit-1 packet is still the limit-50 packet's
// first result byte for byte.
func TestQueryCoverageCountsAdmittedCandidatesPastLimit(t *testing.T) {
	root := evalQueryCoverageRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	const task = "session revocation"
	narrow, err := EvalQuery(context.Background(), index, task, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	wide, err := EvalQuery(context.Background(), index, task, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	narrowCoverage, wideCoverage := evalQueryCoverage(t, narrow), evalQueryCoverage(t, wide)
	admitted := evalQueryCoverageInt(t, wideCoverage, "included_results")
	if admitted < 3 {
		t.Fatalf("fixture admitted %d results at limit 50, want at least 3", admitted)
	}
	if requested := evalQueryCoverageInt(t, wideCoverage, "requested_results"); requested != admitted {
		t.Fatalf("limit-50 requested_results=%d included_results=%d, want equal", requested, admitted)
	}

	t.Run("GPK-V0-040 the selection and its order are unchanged", func(t *testing.T) {
		narrowResults, wideResults := anySlice(narrow["results"]), anySlice(wide["results"])
		if len(narrowResults) != 1 {
			t.Fatalf("limit-1 packet carries %d results, want 1", len(narrowResults))
		}
		first, err := CanonicalJSON(narrowResults[0])
		if err != nil {
			t.Fatal(err)
		}
		expected, err := CanonicalJSON(wideResults[0])
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(expected) {
			t.Fatalf("limit-1 result\n%s\nwant limit-50 first result\n%s", first, expected)
		}
	})

	t.Run("GPK-V0-040 coverage counts the admitted universe, not the output", func(t *testing.T) {
		if requested := evalQueryCoverageInt(t, narrowCoverage, "requested_results"); requested != admitted {
			t.Fatalf("limit-1 requested_results=%d, want the admitted universe %d", requested, admitted)
		}
		if omitted := evalQueryCoverageInt(t, narrowCoverage, "omitted_results"); omitted != admitted-1 {
			t.Fatalf("limit-1 omitted_results=%d, want %d", omitted, admitted-1)
		}
	})

	t.Run("GPK-V0-040 the omission is named in uncertainty", func(t *testing.T) {
		named := false
		for _, line := range evalQueryUncertainty(t, narrowCoverage) {
			if strings.Contains(line, "ranked results omitted by result limit") {
				named = true
			}
		}
		if !named {
			t.Fatalf("limit-1 uncertainty %v names no omission", evalQueryUncertainty(t, narrowCoverage))
		}
	})
}

// TestQueryNamesWithheldTestPathSymbols pins the GPK-V0-052 proposal: a query
// that spells a test declaration's whole name gets an empty packet, and the
// receipt says the candidate was withheld rather than absent.
func TestQueryNamesWithheldTestPathSymbols(t *testing.T) {
	root := evalQueryCoverageRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	const withheldLine = "1 test-path symbol candidates withheld from query ranking; the context verb serves test evidence"

	t.Run("GPK-V0-052 a named test declaration is disclosed, not ranked", func(t *testing.T) {
		receipt, err := EvalQuery(context.Background(), index, "TestSomethingSpecific", 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		if results := anySlice(receipt["results"]); len(results) != 0 {
			t.Fatalf("packet carries %d results, want the ranking unchanged at 0", len(results))
		}
		lines := evalQueryUncertainty(t, evalQueryCoverage(t, receipt))
		if !contains(intersectionSet(lines), withheldLine) {
			t.Fatalf("uncertainty %v does not name the withheld candidate", lines)
		}
	})

	t.Run("GPK-V0-052 a non-test declaration draws no disclosure", func(t *testing.T) {
		receipt, err := EvalQuery(context.Background(), index, "ComputeSessionDigest", 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range evalQueryUncertainty(t, evalQueryCoverage(t, receipt)) {
			if strings.Contains(line, "withheld from query ranking") {
				t.Fatalf("uncertainty names a withheld test candidate for a non-test query: %q", line)
			}
		}
	})
}

// evalQueryFallbackRepository carries one Go file whose declarations share the
// task's whole vocabulary and no records or documents at all, so `competitive`
// is empty and the confident-symbol fallback is the only stage that admits.
func evalQueryFallbackRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore":                ".context-corvint/\n",
		"go.mod":                    "module example.test/digest\n\ngo 1.27.0\n",
		"internal/digest/digest.go": "package digest\n\nfunc ComputeChecksumDigest() string { return \"\" }\n\nfunc VerifyChecksumDigest() bool { return false }\n\nfunc RotateChecksumDigest() string { return \"\" }\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "checksum digest helpers")
	return root
}

// evalQueryFirstResult renders a packet's leading result so two packets can be
// compared byte for byte rather than field by field.
func evalQueryFirstResult(t *testing.T, receipt map[string]any) string {
	t.Helper()
	results := anySlice(receipt["results"])
	if len(results) == 0 {
		t.Fatalf("packet carries no results: %v", receipt["state"])
	}
	encoded, err := CanonicalJSON(results[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// TestQueryCoverageCountsConfidentFallbackPastLimit closes GPK-V0-040's
// remaining branch. When no record competes, the confident-symbol fallback is
// the admitting stage, and bounding it by `--limit` made the coverage block
// report a universe that shrank with the request instead of the one the
// fallback found. The fallback's own caps still bound it; only the caller's
// ceiling stopped doing so, which is why the limit-1 packet is still the
// limit-50 packet's first result byte for byte.
func TestQueryCoverageCountsConfidentFallbackPastLimit(t *testing.T) {
	index, err := Build(context.Background(), evalQueryFallbackRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	const task = "checksum digest"
	narrow, err := EvalQuery(context.Background(), index, task, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	wide, err := EvalQuery(context.Background(), index, task, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	symbols := 0
	for _, raw := range anySlice(wide["results"]) {
		if item, ok := raw.(map[string]any); ok && item["kind"] == "symbol" {
			symbols++
		}
	}
	if symbols < 2 {
		t.Fatalf("fixture's confident fallback supplied %d symbols at limit 50, want at least 2", symbols)
	}
	if requested := evalQueryCoverageInt(t, evalQueryCoverage(t, narrow), "requested_results"); requested < 2 {
		t.Fatalf("limit-1 requested_results=%d, want the fallback's admitted universe of more than one", requested)
	}
	if first, expected := evalQueryFirstResult(t, narrow), evalQueryFirstResult(t, wide); first != expected {
		t.Fatalf("limit-1 result\n%s\nwant limit-50 first result\n%s", first, expected)
	}
}

// TestQueryWithheldDisclosureFitsInsidePacketBudget pins GPK-V0-052's
// disclosure to the same budget every other coverage line answers to. The line
// is appended while results are being fitted, so the packet it lands in is one
// the budget already accounts for; appending it to a finished receipt let a
// packet that had just fitted come back over the budget and say so.
func TestQueryWithheldDisclosureFitsInsidePacketBudget(t *testing.T) {
	index, err := Build(context.Background(), evalQueryCoverageRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	const task = "session revocation TestSomethingSpecific"
	// The budget is a field of the packet it bounds, so its own width moves the
	// size being measured. This settles on the budget the whole packet exactly
	// fills; one byte under that is the only budget where fitting the results
	// with the disclosure and fitting them without it disagree.
	budget, included := MaxPacketBytes, 0
	for step := 0; ; step++ {
		full, err := EvalQuery(context.Background(), index, task, 10, &budget)
		if err != nil {
			t.Fatal(err)
		}
		fullCoverage := evalQueryCoverage(t, full)
		included = evalQueryCoverageInt(t, fullCoverage, "included_results")
		if omitted := evalQueryCoverageInt(t, fullCoverage, "omitted_results"); omitted != 0 {
			t.Fatalf("budget %d already omits %d results before the disclosure is weighed", budget, omitted)
		}
		size := evalQueryCoverageInt(t, fullCoverage, "packet_bytes")
		if size == budget {
			break
		}
		if step == 8 {
			t.Fatalf("packet size did not settle: budget %d, packet_bytes %d", budget, size)
		}
		budget = size
	}
	if included < 2 {
		t.Fatalf("fixture fits %d results at its own size, want at least 2 so one can be displaced", included)
	}
	budget--
	tight, err := EvalQuery(context.Background(), index, task, 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	tightCoverage := evalQueryCoverage(t, tight)
	if within, ok := tightCoverage["within_budget"].(bool); !ok || !within {
		t.Fatalf("within_budget=%v at budget %d, want true", tightCoverage["within_budget"], budget)
	}
	if size := evalQueryCoverageInt(t, tightCoverage, "packet_bytes"); size > budget {
		t.Fatalf("packet_bytes=%d exceeds budget %d", size, budget)
	}
	if omitted := evalQueryCoverageInt(t, tightCoverage, "omitted_results"); omitted < 1 {
		t.Fatalf("omitted_results=%d, want the displaced result counted", omitted)
	}
	if fitted := evalQueryCoverageInt(t, tightCoverage, "included_results"); fitted >= included {
		t.Fatalf("included_results=%d at budget %d, want fewer than the %d that fit unconstrained", fitted, budget, included)
	}
	lines := evalQueryUncertainty(t, tightCoverage)
	named, disclosed := false, false
	for _, line := range lines {
		if strings.Contains(line, "ranked results omitted by packet budget") {
			named = true
		}
		if strings.Contains(line, "withheld from query ranking") {
			disclosed = true
		}
	}
	if !named || !disclosed {
		t.Fatalf("uncertainty %v, want both the budget omission and the withheld disclosure", lines)
	}
}

// evalQueryIdentifierRepository indexes two test declarations whose names a
// stemmed or split task term would reach but a quoted identifier would not.
func evalQueryIdentifierRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore":                     ".context-corvint/\n",
		"go.mod":                         "module example.test/limits\n\ngo 1.27.0\n",
		"internal/limits/limits.go":      "package limits\n\nfunc Ceiling() int { return 0 }\n",
		"internal/limits/limits_test.go": "package limits\n\nfunc max() int { return 0 }\n\nfunc TestFoo_Bar() {}\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "limits and their test declarations")
	return root
}

// TestQueryDisclosesOnlyTaskNamedTestDeclarations pins GPK-V0-052's match to
// the identifiers the task actually wrote. The disclosure claims the task
// spelled a declaration's whole name, so it reads unexpanded tokens: the
// relevance terms the ranker runs on are stemmed and split, and matching there
// let `maximum` disclose a `max` helper the task never named.
func TestQueryDisclosesOnlyTaskNamedTestDeclarations(t *testing.T) {
	index, err := Build(context.Background(), evalQueryIdentifierRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	disclosures := func(t *testing.T, task string) []string {
		t.Helper()
		receipt, err := EvalQuery(context.Background(), index, task, 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		named := make([]string, 0)
		for _, line := range evalQueryUncertainty(t, evalQueryCoverage(t, receipt)) {
			if strings.Contains(line, "withheld from query ranking") {
				named = append(named, line)
			}
		}
		return named
	}

	t.Run("GPK-V0-052 an expanded term does not name a declaration", func(t *testing.T) {
		if named := disclosures(t, "maximum"); len(named) != 0 {
			t.Fatalf("task %q discloses %v, want nothing: it names no declaration", "maximum", named)
		}
	})

	t.Run("GPK-V0-052 an underscored identifier is one token", func(t *testing.T) {
		named := disclosures(t, "TestFoo_Bar")
		if len(named) != 1 {
			t.Fatalf("task %q discloses %v, want one withheld candidate", "TestFoo_Bar", named)
		}
		if !strings.HasPrefix(named[0], "1 test-path symbol candidates withheld") {
			t.Fatalf("disclosure %q does not count exactly one withheld candidate", named[0])
		}
	})
}

// TestQueryAdmitsThreeImplementationsPerFeature pins the per-feature class cap
// GPK-V0-040 denominates (decision 0157): a competitive feature admits its
// three strongest implementations whatever the limit, and not a fourth.
func TestQueryAdmitsThreeImplementationsPerFeature(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "go.mod", "module example.test/featurecap\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n  - id: ledger-rotation\n    area: ledger\n    summary: Rotate the ledger.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios:\n  - id: ledger-rotation-rotates\n    area: ledger\n    summary: The ledger rotates.\n    features: [ledger-rotation]\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "internal/ledger/rotation.go", "package ledger\n\nfunc LedgerRotation() {}\n\nfunc LedgerRotationPlan() {}\n\nfunc LedgerRotationAudit() {}\n\nfunc LedgerRotationWindow() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "ledger rotation")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	implementations := featureImplementationCandidates(index, index.Features["ledger-rotation"], 4)
	if len(implementations) != 4 {
		t.Fatalf("fixture ranks %d implementations, want 4", len(implementations))
	}
	receipt, err := EvalQuery(context.Background(), index, "ledger rotation", 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	emitted := make(map[string]bool)
	for _, result := range anySlice(receipt["results"]) {
		emitted[stringValue(result.(map[string]any)["id"])] = true
	}
	for rank, implementation := range implementations {
		id := stringValue(implementation["id"])
		if emitted[id] != (rank < 3) {
			t.Errorf("implementation rank %d %s emitted=%v, want %v", rank+1, id, emitted[id], rank < 3)
		}
	}
}
