package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCP-V1-257: the eval command serves the JSON report, the table view and the baseline
// ratchet; a regression exits 1 with the report and an unknown view exits 2.
func TestDCPV1257CorpusEvalCLI(t *testing.T) {
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	cemWrite(t, root, "cases.json", `{"schema":"corvint-corpus-eval-cases/1","origin":"labelled","cases":[
{"id":"locate-demux","operation":"locate","arguments":{"path":"cache/demux.go"},"expect":{"paths":["cache/demux.go"]},"tags":["cache"]},
{"id":"missing","operation":"search","arguments":{"query":"Split"},"expect":{"paths":["cache/missing.go"]}},
{"id":"absent","operation":"search","arguments":{"query":"zebraquux"},"expect":{"abstain":true}}]}`)
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "eval", "--artifact", "corpus.json", "--cases", "cases.json")
	if code != 0 {
		t.Fatalf("eval = %d %s", code, stderr)
	}
	var report corpusbridge.EvalReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Schema != corpusbridge.EvalReceiptSchema || len(report.Cases) != 3 || report.Cases[0].Status != "pass" || report.Cases[1].Status != "fail" || len(report.Cases[1].Unresolved) != 1 || report.Cases[1].Unresolved[0].ID != "cache/missing.go" || report.Cases[2].Abstention != "correct" {
		t.Fatalf("report = %s", out)
	}
	code, table, stderr := corpusCLI(t, root, "docs", "corpus", "eval", "--artifact", "corpus.json", "--cases", "cases.json", "--format", "table")
	if code != 0 || !strings.Contains(table, "locate-demux") || !strings.Contains(table, "unresolved") {
		t.Fatalf("table = %d %q %s", code, table, stderr)
	}
	cemWrite(t, root, "baseline.json", out)
	if code, _, stderr := corpusCLI(t, root, "docs", "corpus", "eval", "--artifact", "corpus.json", "--cases", "cases.json", "--baseline", "baseline.json"); code != 0 {
		t.Fatalf("unchanged baseline = %d %s", code, stderr)
	}
	// A baseline in which the missing case passed: the current run regresses it.
	var raw map[string]any
	_ = json.Unmarshal([]byte(out), &raw)
	missing := raw["cases"].([]any)[1].(map[string]any)
	missing["status"] = "pass"
	missing["unresolved"] = []any{}
	edited, _ := json.Marshal(raw)
	cemWrite(t, root, "baseline.json", string(edited))
	code, out, _ = corpusCLI(t, root, "docs", "corpus", "eval", "--artifact", "corpus.json", "--cases", "cases.json", "--baseline", "baseline.json")
	var regressed corpusbridge.EvalReport
	if err := json.Unmarshal([]byte(out), &regressed); err != nil {
		t.Fatal(err)
	}
	if code != 1 || regressed.Baseline == nil || len(regressed.Baseline.Regressed) != 1 || regressed.Baseline.Regressed[0].ID != "missing" {
		t.Fatalf("regression = %d %s", code, out)
	}
	for _, args := range [][]string{
		{"--format", "yaml"},
		{"--cases", "absent.json"},
	} {
		base := []string{"docs", "corpus", "eval", "--artifact", "corpus.json", "--cases", "cases.json"}
		if args[0] == "--cases" {
			base = base[:len(base)-2]
		}
		if code, _, _ := corpusCLI(t, root, append(base, args...)...); code != 2 {
			t.Fatalf("%v exit = %d, want 2", args, code)
		}
	}
	if code, _, _ := corpusCLI(t, root, "docs", "corpus", "eval", "--artifact", "corpus.json"); code != 2 {
		t.Fatalf("missing --cases exit = %d", code)
	}
}
