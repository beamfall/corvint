package gateagreement

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedClock(t *testing.T) {
	t.Helper()
	previous := Now
	Now = func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { Now = previous })
}

// TestGVAV0003CompareSortsOutcomes proves findings and verdicts sort into agreements,
// Corvint-only, caller-only and abstentions, a line-free verdict matches any line, a pair is
// unordered, and the per-rule counts follow (GVA-V0-002, GVA-V0-003, GVA-V0-004).
func TestGVAV0003CompareSortsOutcomes(t *testing.T) {
	fixedClock(t)
	verdicts, err := ParseVerdicts("v.json", []byte(`{"schema":"corvint-gate-verdicts/0","verdicts":[
 {"rule":"fixed-wait","path":"e2e/a.spec.ts","line":7},
 {"rule":"no-assertion","path":"e2e/a.spec.ts"},
 {"rule":"fixed-wait","path":"e2e/b.spec.ts","line":3},
 {"rule":"fixed-wait","path":"e2e/odd.spec.ts","line":1},
 {"rule":"duplicate-pair","test":"e2e/b.spec.ts :: y","other":"e2e/a.spec.ts :: x"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	observed := Observed{
		Findings: []Key{{Rule: "fixed-wait", Path: "e2e/a.spec.ts", Line: 7}, {Rule: "no-assertion", Path: "e2e/a.spec.ts", Line: 4},
			{Rule: "brittle-selector", Path: "e2e/a.spec.ts", Line: 9}, {Rule: "duplicate-pair", Test: "e2e/a.spec.ts :: x", Other: "e2e/b.spec.ts :: y"}},
		Abstentions: []Abstention{{Key: Key{Path: "e2e/odd.spec.ts"}, Reason: "file-uncertain"}},
	}
	record := Compare(Run{Command: "tests lint", Arguments: []string{"--base", "x"}, Build: map[string]string{"version": "v"}, Revision: "abc", ExitCode: 1, Advisory: true}, observed, &verdicts)
	if record.Comparison != StatusCompared || record.AdvisoryExitCode != 0 || record.ExitCode != 1 || record.At != "2026-10-27T12:00:00Z" ||
		!strings.HasPrefix(record.Verdicts.SHA256, "sha256:") {
		t.Fatalf("record = %+v", record)
	}
	if len(record.Agreements) != 3 || len(record.CorvintOnly) != 1 || record.CorvintOnly[0].Rule != "brittle-selector" ||
		len(record.CallerOnly) != 1 || record.CallerOnly[0].Path != "e2e/b.spec.ts" ||
		len(record.Abstentions) != 1 || record.Abstentions[0].Reason != "file-uncertain" {
		t.Fatalf("outcomes = %+v", record)
	}
	if got := record.Rules["fixed-wait"]; got != (RuleCounts{Agreements: 1, CallerOnly: 1, Abstentions: 1}) {
		t.Fatalf("fixed-wait counts = %+v", got)
	}
	none := Compare(Run{Command: "tests lint", ExitCode: 2, Advisory: true}, observed, nil)
	if none.Comparison != StatusNotProduced || none.Reason != "no-verdict-file" || none.AdvisoryExitCode != 2 || len(none.Rules) != 0 {
		t.Fatalf("no verdicts = %+v", none)
	}
}

// TestGVAV0002VerdictFileRefusals proves a malformed verdict file refuses (GVA-V0-002).
func TestGVAV0002VerdictFileRefusals(t *testing.T) {
	for _, text := range []string{`{"schema":"other","verdicts":[]}`, `{"schema":"corvint-gate-verdicts/0","verdicts":[],"x":1}`,
		`{"schema":"corvint-gate-verdicts/0","verdicts":[{"rule":"","path":"a"}]}`, `{"schema":"corvint-gate-verdicts/0","verdicts":[{"rule":"r"}]}`} {
		if _, err := ParseVerdicts("v.json", []byte(text)); err == nil || !strings.Contains(err.Error(), "agreement-verdicts-invalid") {
			t.Fatalf("%s: %v", text, err)
		}
	}
}

// TestGVAV0007SummaryRates proves the summary appends and reads records, rates agreement per rule
// over compared runs only, filters by command and lists a malformed line (GVA-V0-005,
// GVA-V0-007, GVA-V0-008).
func TestGVAV0007SummaryRates(t *testing.T) {
	fixedClock(t)
	log := filepath.Join(t.TempDir(), "a.jsonl")
	verdicts := &Verdicts{Name: "v", SHA256: "sha256:x", Verdicts: []Key{{Rule: "r", Path: "a", Line: 1}, {Rule: "r", Path: "b", Line: 1}}}
	runs := []Observed{{Findings: []Key{{Rule: "r", Path: "a", Line: 1}}}, {Findings: []Key{{Rule: "r", Path: "a", Line: 1}, {Rule: "s", Path: "c", Line: 2}}}}
	for _, observed := range runs {
		if err := Append(log, Compare(Run{Command: "tests lint", ExitCode: 1}, observed, verdicts)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Append(log, Compare(Run{Command: "tests lint"}, runs[0], nil)); err != nil {
		t.Fatal(err)
	}
	if err := Append(log, Compare(Run{Command: "migration-ratchet"}, runs[0], verdicts)); err != nil {
		t.Fatal(err)
	}
	file, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = file.WriteString("not json\n")
	_ = file.Close()
	if info, _ := os.Stat(log); info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %v", info.Mode())
	}
	open := func(name string) (io.ReadCloser, error) { return os.Open(name) }
	summary, err := Summarise([]string{log}, "tests lint", open)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Runs != 3 || summary.ComparedRuns != 2 || len(summary.IgnoredLines) != 1 || summary.IgnoredLines[0].Line != 5 || len(summary.Rules) != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	r := summary.Rules[0]
	if r.Rule != "r" || r.Runs != 2 || r.Agreements != 2 || r.CallerOnly != 2 || r.AgreementRate == nil || *r.AgreementRate != 0.5 {
		t.Fatalf("rule r = %+v", r)
	}
	if s := summary.Rules[1]; s.Rule != "s" || s.CorvintOnly != 1 || *s.AgreementRate != 0 {
		t.Fatalf("rule s = %+v", s)
	}
	all, _ := Summarise([]string{log}, "", open)
	if all.Runs != 4 || all.Commands["migration-ratchet"] != 1 {
		t.Fatalf("all = %+v", all)
	}
	if _, err := Summarise([]string{log + ".absent"}, "", open); err == nil || !strings.Contains(err.Error(), "agreement-log-unavailable") {
		t.Fatalf("missing log: %v", err)
	}
	if err := Append(t.TempDir(), Record{}); err == nil {
		t.Fatal("append to a directory succeeded")
	}
}
