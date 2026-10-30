package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func receiptLine(t *testing.T, task, arm string, resolved bool, cost float64, complete bool) string {
	t.Helper()
	r := map[string]any{"profile": receiptProfile, "task": task, "arm": arm, "resolved": resolved, "cost_usd": cost}
	if complete {
		frame := map[string]any{"bytes": 0}
		if arm == "on" {
			frame = map[string]any{"bytes": 412, "sha256": strings.Repeat("a", 64), "base_tree": "abc"}
		}
		r["frame"] = frame
		r["steps"] = []any{map[string]any{"tool": "read", "input_tokens": 10, "cost_usd": cost / 2}, map[string]any{"tool": "edit", "cost_usd": cost / 2}}
		r["grader"] = map[string]any{"tests": []any{map[string]any{"name": "TestX", "outcome": map[bool]string{true: "pass", false: "fail"}[resolved]}}}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func runOn(t *testing.T, lines []string, extra ...string) (report, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "receipts.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := run(append([]string{"--receipts", path, "--resamples", "2000"}, extra...), &out)
	var rep report
	if err == nil {
		if decodeErr := json.Unmarshal(out.Bytes(), &rep); decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	return rep, err
}

// AHI-217 (V1-2756): per-pair deltas, the pooled cost ratio with a bootstrap interval, and an
// exact McNemar test on resolution; missing frame, ledger and grader fields are counted, never
// silently dropped, and --require-complete refuses them.
func TestAHI217PairedTrialAnalysis(t *testing.T) {
	t.Parallel()
	var lines []string
	for i, task := range []string{"p01", "p02", "p03", "p04", "p05", "p06", "p07", "p08", "p09", "p10"} {
		onResolved, offResolved := task != "p07", task != "p05"
		lines = append(lines, receiptLine(t, task, "off", offResolved, 0.25+0.01*float64(i), true), receiptLine(t, task, "on", onResolved, 0.28+0.01*float64(i), true))
	}
	rep, err := runOn(t, lines, "--require-complete")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Pairs) != 10 || rep.McNemar.OnOnly != 1 || rep.McNemar.OffOnly != 1 || rep.McNemar.Both != 8 || rep.McNemar.PExact != 1 {
		t.Fatalf("pairs/mcnemar: %+v", rep.McNemar)
	}
	if p := rep.Pairs[0]; p.Task != "p01" || math.Abs(p.CostDeltaUSD-0.03) > 1e-9 || p.Flip != "both" {
		t.Fatalf("first pair %+v", p)
	}
	want := (2.8 + 0.45) / (2.5 + 0.45)
	if math.Abs(rep.CostRatio-want) > 1e-9 || rep.CostRatioCI[0] > rep.CostRatio || rep.CostRatioCI[1] < rep.CostRatio || rep.CostRatioCI[0] >= rep.CostRatioCI[1] {
		t.Fatalf("ratio %v ci %v want %v", rep.CostRatio, rep.CostRatioCI, want)
	}
	again, _ := runOn(t, lines)
	if again.CostRatioCI != rep.CostRatioCI {
		t.Fatal("a fixed seed must reproduce the interval")
	}
	for field, n := range rep.NotObserved {
		if n != 0 {
			t.Fatalf("complete receipts reported %s NOT_OBSERVED", field)
		}
	}

	// Six discordant pairs all one way: exact two-sided p = 2/64.
	var oneWay []string
	for _, task := range []string{"a", "b", "c", "d", "e", "f"} {
		oneWay = append(oneWay, receiptLine(t, task, "off", true, 1, true), receiptLine(t, task, "on", false, 1, true))
	}
	if rep, err := runOn(t, oneWay); err != nil || math.Abs(rep.McNemar.PExact-2.0/64) > 1e-12 || rep.CostRatioCI != [2]float64{1, 1} {
		t.Fatalf("one-way: %+v %v %v", rep.McNemar, rep.CostRatioCI, err)
	}

	// Incomplete receipts are counted as NOT_OBSERVED; an unpaired task and a ledger that does not
	// sum to the session cost are named; --require-complete refuses.
	mismatch := strings.Replace(receiptLine(t, "x", "on", true, 1, true), `"cost_usd":1,`, `"cost_usd":2,`, 1)
	partial := []string{receiptLine(t, "x", "off", true, 1, false), mismatch, receiptLine(t, "lonely", "on", true, 1, true)}
	rep, err = runOn(t, partial)
	if err != nil {
		t.Fatal(err)
	}
	if rep.NotObserved["frame"] != 1 || rep.NotObserved["steps"] != 1 || rep.NotObserved["grader_tests"] != 1 || len(rep.Unpaired) != 1 || rep.Unpaired[0] != "lonely#0" || len(rep.LedgerMismatches) != 1 {
		t.Fatalf("partial: %+v unpaired %v mismatches %v", rep.NotObserved, rep.Unpaired, rep.LedgerMismatches)
	}
	if _, err := runOn(t, partial, "--require-complete"); err == nil || !strings.Contains(err.Error(), "do not record") {
		t.Fatalf("require-complete accepted incomplete receipts: %v", err)
	}
	noBase := strings.Replace(receiptLine(t, "y", "on", true, 1, true), `"base_tree":"abc",`, "", 1)
	if _, err := runOn(t, []string{receiptLine(t, "y", "off", true, 1, true), noBase}, "--require-complete"); err == nil || !strings.Contains(err.Error(), "frame_base_tree") {
		t.Fatalf("a frame without its base tree was accepted: %v", err)
	}

	// Malformed inputs are refused, not guessed.
	for name, bad := range map[string][]string{
		"duplicate":  {receiptLine(t, "z", "on", true, 1, true), receiptLine(t, "z", "on", true, 1, true)},
		"profile":    {strings.Replace(receiptLine(t, "z", "on", true, 1, true), receiptProfile, "other/0", 1)},
		"no pairs":   {receiptLine(t, "z", "on", true, 1, true)},
		"zero off":   {receiptLine(t, "z", "off", true, 0, false), receiptLine(t, "z", "on", true, 1, false)},
		"no outcome": {strings.Replace(receiptLine(t, "z", "on", true, 1, false), `"resolved":true,`, "", 1)},
	} {
		if _, err := runOn(t, bad); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}
