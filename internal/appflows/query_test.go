package appflows

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFU-V1-016 AFU-V1-036 AFU-V1-037
func TestAFUV1ReadRunEvidenceDiscipline(t *testing.T) {
	dir := t.TempDir()
	static := TestRunEvidence{Schema: RunEvidenceSchema, Authority: AuthorityStatic, TestKey: "k", Attempts: []RunAttempt{}, Cleanup: "not-declared", NegativeControls: []NegativeControl{}}
	line, err := EncodeRunEvidence(static)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("good.jsonl", bytes.Repeat(line, 2))
	if got, err := ReadRunEvidence([]string{good, good}); err != nil || len(got) != 4 {
		t.Fatalf("canonical records: %d %v", len(got), err)
	}
	spaced := write("spaced.jsonl", bytes.Replace(line, []byte(`,"`), []byte(`, "`), 1))
	if err := os.Symlink(good, filepath.Join(dir, "link.jsonl")); err != nil {
		t.Fatal(err)
	}
	over := write("over.jsonl", bytes.Repeat(line, maxRunRecords/2+1))
	for name, files := range map[string][]string{"non-canonical": {spaced}, "symlink": {filepath.Join(dir, "link.jsonl")}, "missing": {filepath.Join(dir, "none")}} {
		if _, err := ReadRunEvidence(files); err == nil {
			t.Fatalf("%s evidence accepted", name)
		}
	}
	if _, err := ReadRunEvidence([]string{over, over}); err == nil || !strings.Contains(err.Error(), BoundRecords) {
		t.Fatalf("combined record bound: %v", err)
	}
}

// AFU-V1-014 AFU-V1-016
func TestAFUV1EvidenceStateOrder(t *testing.T) {
	at := head{commit: strings.Repeat("1", 40), tree: strings.Repeat("2", 40)}
	attempt := func(n int, outcome string) RunAttempt {
		return RunAttempt{Ordinal: n, Outcome: outcome, AssertionAnchors: []RunAnchor{}, Attachments: []RunAttachment{}}
	}
	record := func(edit func(*TestRunEvidence)) TestRunEvidence {
		h := runHeader()
		r := TestRunEvidence{Schema: RunEvidenceSchema, Authority: AuthorityIngested, RunID: h.RunID, Runner: RunRunner{Name: "go-test", Version: h.RunnerVersion},
			Source: h.Source, BuildArtifactDigest: h.BuildArtifactDigest, Environment: h.Environment, Fixture: h.Fixture, TestKey: "k",
			Attempts: []RunAttempt{attempt(1, "passed")}, Cleanup: "done", NegativeControls: []NegativeControl{{TestKey: "c", Expected: "failed", Observed: "failed"}}}
		edit(&r)
		return r
	}
	cases := []struct {
		state   string
		records []TestRunEvidence
	}{
		{"missing", nil},
		{"missing", []TestRunEvidence{record(func(r *TestRunEvidence) { r.Authority = AuthorityStatic })}},
		{"stale", []TestRunEvidence{record(func(r *TestRunEvidence) { r.Source.Commit = strings.Repeat("3", 40) })}},
		{"stale", []TestRunEvidence{record(func(r *TestRunEvidence) { r.Source.Clean = false })}},
		{"flaky", []TestRunEvidence{record(func(*TestRunEvidence) {}), record(func(r *TestRunEvidence) {
			r.Attempts = []RunAttempt{attempt(1, "failed"), attempt(2, "passed")}
		})}},
		{"failed", []TestRunEvidence{record(func(r *TestRunEvidence) { r.Attempts = []RunAttempt{attempt(1, "failed")} })}},
		{"negative-control-missing", []TestRunEvidence{record(func(r *TestRunEvidence) { r.NegativeControls = []NegativeControl{} })}},
		{"negative-control-missing", []TestRunEvidence{record(func(r *TestRunEvidence) { r.NegativeControls[0].Observed = "passed" })}},
		{"cleanup-unverified", []TestRunEvidence{record(func(r *TestRunEvidence) { r.Cleanup = "failed" })}},
		{"verified", []TestRunEvidence{record(func(*TestRunEvidence) {})}},
	}
	for _, c := range cases {
		if got := evidenceState(c.records, []string{"c"}, at); got != c.state {
			t.Fatalf("want %s, got %s", c.state, got)
		}
	}
	if evidenceGaps["failed"] != GapEvidenceMissing || evidenceGaps["verified"] != "" {
		t.Fatal("evidence state to gap code mapping changed")
	}
}
