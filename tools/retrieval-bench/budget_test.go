package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCEPV0008EqualBudgetWindowsScoreEveryArmAlike: under one byte budget
// every arm reads the same text window per ranked file, so a Corvint ranking
// that spends the budget on a non-gold file loses to a baseline that ranks
// the gold file first, whatever their packet sizes; the historical metrics
// are unchanged and a run without the flag carries no budget field.
func TestCEPV0008EqualBudgetWindowsScoreEveryArmAlike(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	answers := map[string]arm{
		"Quote empty default": {Ranked: []string{"src/help.rs", "tests/help_test.rs"}, State: "READY", PacketBytes: 9},
		"quote default value": {Ranked: []string{"src/help.rs", "src/format.rs"}, State: "READY"},
		"help output":         {Ranked: []string{"README.md"}, State: "READY"},
	}
	configuration := fixtureOptions(t, fixtureSnapshot(t))
	configuration.arms = map[string]bool{"corvint": true, "grep": true}
	configuration.budgetBytes, configuration.windowBytes = 40, 30
	report, err := bench(context.Background(), configuration, fakeCorvint(answers), fakeCorvint(answers), fakeImpact(nil), fakeAffected(nil))
	if err != nil {
		t.Fatal(err)
	}
	budget, _ := report["budget"].(map[string]any)
	if budget["budget_bytes"] != 40 || budget["window_bytes"] != 30 || budget["tokens"] != tokenBudgetState {
		t.Fatalf("budget block = %v", report["budget"])
	}
	if registered := report["registration"].(map[string]any); registered["budget"] == nil {
		t.Fatalf("registration does not bind the budget: %v", registered)
	}
	byID := map[string]sampleReport{}
	for _, detail := range report["details"].([]sampleReport) {
		byID[detail.ID] = detail
	}
	positive := byID["pos"]
	// src/help.rs costs 28 bytes; tests/help_test.rs's 30-byte window would
	// reach 58 > 40, so the Corvint arm delivers one non-gold window.
	if got := positive.Metrics["corvint"]; got["budget_recall"] != 0 || got["budget_files"] != 1 || got["budget_bytes_used"] != 28 || got["recall@5"] != 1 {
		t.Fatalf("corvint budget metrics = %v", got)
	}
	if got := positive.Metrics["grep"]; got["budget_recall"] != 1 || got["budget_hit"] != 1 || got["budget_bytes_used"] > 40 {
		t.Fatalf("grep budget metrics = %v (ranked %v)", got, positive.Arms["grep"].Ranked)
	}
	if got := byID["nat"].Metrics["corvint"]; got["budget_files"] != 0 {
		t.Fatalf("an abstention delivered text: %v", got)
	}
	if _, present := byID["nat"].Metrics["corvint"]["budget_recall"]; present {
		t.Fatal("a no-gold sample carries budget recall")
	}
	pairs := report["paired"].(map[string]any)["all"].(map[string]any)["corvint"].(map[string]any)["grep"].(map[string]any)
	if pairs["budget_recall"] == nil {
		t.Fatalf("paired block lacks budget recall: %v", pairs)
	}

	configuration.budgetBytes = 0
	plain, err := bench(context.Background(), configuration, fakeCorvint(answers), fakeCorvint(answers), fakeImpact(nil), fakeAffected(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["budget"]; present {
		t.Fatal("a run without --budget-bytes reports a budget")
	}
	for _, detail := range plain["details"].([]sampleReport) {
		for name, scored := range detail.Metrics {
			if _, present := scored["budget_recall"]; present {
				t.Fatalf("%s/%s carries a budget metric without the flag", detail.ID, name)
			}
		}
	}
}

// TestCEPV0009OutcomesSeparateCoverageAbstentionErrorsAndAnsweredPrecision:
// an abstention or an error never moves answered-only precision, and the
// historical mean that scores them as zero is kept beside it.
func TestCEPV0009OutcomesSeparateCoverageAbstentionErrorsAndAnsweredPrecision(t *testing.T) {
	reports := []sampleReport{
		{ID: "answered", Stratum: "positive", Arms: map[string]arm{"context": {Ranked: []string{"a", "b"}}}, Metrics: map[string]metrics{"context": {"precision@k": 0.5, "hit@k": 1}}},
		{ID: "abstained", Stratum: "positive", Arms: map[string]arm{"context": {Abstained: true}}, Metrics: map[string]metrics{"context": {"precision@k": 0, "hit@k": 0, "abstained": 1}}},
		{ID: "errored", Stratum: "positive", Arms: map[string]arm{"context": {Error: "exit status 2"}}, Metrics: map[string]metrics{"context": {"precision@k": 0, "hit@k": 0}}},
		{ID: "nogold", Stratum: "natural_no_gold", Arms: map[string]arm{"context": {Ranked: []string{"c"}}}, Metrics: map[string]metrics{"context": {"selective_success": 0}}},
	}
	summary := summarize(reports, 5)["context"].(map[string]any)
	all := summary["outcomes"].(map[string]any)["all"].(map[string]any)
	if all["n"] != 4 || all["answered"] != 2 || all["abstained"] != 1 || all["errors"] != 1 {
		t.Fatalf("outcome counts = %v", all)
	}
	if all["coverage"] != 0.5 || all["abstention_rate"] != 0.25 || all["error_rate"] != 0.25 {
		t.Fatalf("outcome rates = %v", all)
	}
	if all["answered_positives"] != 1 || all["answered_precision@k"] != 0.5 {
		t.Fatalf("answered-only precision = %v", all)
	}
	if historical := summary["all"].(map[string]any)["precision@k"]; historical != round(0.5/3) {
		t.Fatalf("historical precision@k = %v", historical)
	}
	empty := outcomes(nil, "context")
	if empty["coverage"] != nil || empty["answered_precision@k"] != nil {
		t.Fatalf("an empty group reports a conventional rate: %v", empty)
	}
}

// TestCEPV0010ReportNamesItsExperimentKindAndBudgetFlagsAreChecked: the
// report is a retrieval experiment only; oracle-context utility, tool
// adoption and the official downstream grader are NOT_RUN here.
func TestCEPV0010ReportNamesItsExperimentKindAndBudgetFlagsAreChecked(t *testing.T) {
	kinds := experiment()
	if kinds["kind"] != "retrieval" {
		t.Fatalf("kind = %v", kinds["kind"])
	}
	for _, key := range []string{"oracle_context", "tool_adoption", "downstream_grader"} {
		if value, _ := kinds[key].(string); !strings.HasPrefix(value, "NOT_RUN:") {
			t.Fatalf("%s = %v", key, kinds[key])
		}
	}
	for _, arguments := range [][]string{
		{"--samples", "s", "--corpus", "c", "--budget-bytes", "-1"},
		{"--samples", "s", "--corpus", "c", "--budget-bytes", "100", "--window-bytes", "0"},
		{"--summarize", "r.json", "--budget-bytes", "100"},
	} {
		if _, err := parseOptions(arguments); err == nil {
			t.Fatalf("parseOptions(%v) accepted", arguments)
		}
	}
	configuration, err := parseOptions([]string{"--samples", "s", "--corpus", "c", "--budget-bytes", "8192"})
	if err != nil || configuration.windowBytes != defaultWindowBytes {
		t.Fatalf("options = %+v, %v", configuration, err)
	}
}

// TestCEPV0008AbstainedOrErroredArmDeliversNoWindow: an OUT_OF_SCOPE packet
// that still ranks widening paths is an abstention, so like an errored arm
// it spends no budget and earns no budget recall or hit.
func TestCEPV0008AbstainedOrErroredArmDeliversNoWindow(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gold.go"), []byte("package gold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configuration := options{budgetBytes: 100, windowBytes: 50}
	for name, answer := range map[string]arm{
		"abstained": {Ranked: []string{"gold.go"}, State: "OUT_OF_SCOPE", Abstained: true},
		"errored":   {Ranked: []string{"gold.go"}, Error: "boom"},
	} {
		result := metrics{}
		budgetMetrics(result, answer, []string{"gold.go"}, root, true, configuration)
		if result["budget_files"] != 0 || result["budget_bytes_used"] != 0 || result["budget_recall"] != 0 || result["budget_hit"] != 0 {
			t.Errorf("%s arm delivered text: %v", name, result)
		}
	}
	result := metrics{}
	budgetMetrics(result, arm{Ranked: []string{"gold.go"}, State: "READY"}, []string{"gold.go"}, root, true, configuration)
	if result["budget_files"] != 1 || result["budget_recall"] != 1 {
		t.Fatalf("answered arm = %v", result)
	}
}

// TestCEPV0008SummarizeRefusesMixedBudgetsAndKeepsTheBudget: budget_recall
// is paired only at one budget, so --summarize refuses reports whose budget
// blocks differ (including one with and one without a budget) and carries
// the shared budget into the merged report and its registration.
func TestCEPV0008SummarizeRefusesMixedBudgetsAndKeepsTheBudget(t *testing.T) {
	directory := t.TempDir()
	writeReport := func(name, budget, arm string) string {
		path := filepath.Join(directory, name)
		block := ""
		if budget != "" {
			block = `"budget":` + budget + `,`
		}
		body := fmt.Sprintf(`{"samples_sha256":"s","limit":5,%s"corvint":{"sha256":"NOT_RUN"},"details":[{"id":"a","repo":"gin-gonic/gin","stratum":"positive","arms":{%q:{"ranked":["x"]}},"metrics":{%q:{"recall@10":1,"budget_recall":1}}}]}`, block, arm, arm)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	small := `{"unit":"bytes","budget_bytes":4096,"window_bytes":1024,"tokens":"NOT_MEASURED"}`
	large := `{"unit":"bytes","budget_bytes":8192,"window_bytes":1024,"tokens":"NOT_MEASURED"}`
	grep := writeReport("grep.json", small, "grep")
	bm25 := writeReport("bm25.json", small, "bm25:all")
	wider := writeReport("wider.json", large, "bm25:all")
	none := writeReport("none.json", "", "bm25:all")
	for _, mixed := range [][]string{{grep, wider}, {grep, none}, {none, grep}} {
		if _, err := resummarize(options{summaries: mixed}); err == nil || !strings.Contains(err.Error(), "budget") {
			t.Errorf("mixed budgets %s + %s merged: %v", filepath.Base(mixed[0]), filepath.Base(mixed[1]), err)
		}
	}
	report, err := resummarize(options{summaries: []string{grep, bm25}})
	if err != nil {
		t.Fatal(err)
	}
	budget, _ := report["budget"].(map[string]any)
	if budget["budget_bytes"] != float64(4096) || budget["window_bytes"] != float64(1024) || report["registration"].(map[string]any)["budget"] == nil {
		t.Fatalf("merged report lost the budget: budget=%v registration=%v", report["budget"], report["registration"])
	}
	plain, err := resummarize(options{summaries: []string{none}})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["budget"]; present {
		t.Fatal("a merge of unbudgeted reports declares a budget")
	}
}
