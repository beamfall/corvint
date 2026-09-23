package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CEP-V0-001: a ContextBench row maps to one sample with ContextBench's own
// path normalization, and a row it cannot map is refused.
func TestContextBenchRowsMapToSamplesAndBadRowsAreRefused(t *testing.T) {
	for path, want := range map[string]string{
		"/testbed/a/b.py": "a/b.py", "/workspace/repo/a/b.py": "a/b.py", "/workspace/b.py": "b.py",
		"/a/b.py": "a/b.py", "./a/b.py": "a/b.py", "../a/b.py": "a/b.py", ".github/x.yml": "github/x.yml", `a\b.py`: "a/b.py",
	} {
		if got := contextBenchPath(path); got != want {
			t.Errorf("contextBenchPath(%q) = %q, want %q", path, got, want)
		}
	}
	samples, digest, _, err := readSamples(options{samples: filepath.Join("testdata", "contextbench", "rows.jsonl")})
	if err != nil {
		t.Fatal(err)
	}
	first := samples[0]
	if len(samples) != 2 || len(digest) != 64 || first.TaskType != contextBenchTask || stratum(first) != "positive" {
		t.Fatalf("samples: %d %q %+v", len(samples), digest, first)
	}
	if strings.Join(goldFiles(first), ",") != "ring.go,ring_test.go" || !strings.HasPrefix(queryText(first), "Ring.Len reports") {
		t.Fatalf("gold %v query %q", goldFiles(first), queryText(first))
	}
	bad := map[string]string{
		"no statement":  `{"instance_id":"x","repo":"o/n","base_commit":"c","gold_context":"[]"}`,
		"gold not json": `{"instance_id":"x","repo":"o/n","base_commit":"c","problem_statement":"p","gold_context":"[{"}`,
		"inverted span": `{"instance_id":"x","repo":"o/n","base_commit":"c","problem_statement":"p","gold_context":"[{\"file\":\"a.py\",\"start_line\":5,\"end_line\":4}]"}`,
		"zero line":     `{"instance_id":"x","repo":"o/n","base_commit":"c","problem_statement":"p","gold_context":"[{\"file\":\"a.py\",\"start_line\":0,\"end_line\":4}]"}`,
		"line past cap": `{"instance_id":"x","repo":"o/n","base_commit":"c","problem_statement":"p","gold_context":"[{\"file\":\"a.py\",\"start_line\":1,\"end_line\":9223372036854775807},{\"file\":\"b.py\",\"start_line\":1,\"end_line\":9223372036854775807}]"}`,
		"fractional":    `{"instance_id":"x","repo":"o/n","base_commit":"c","problem_statement":"p","gold_context":"[{\"file\":\"a.py\",\"start_line\":1.5,\"end_line\":4}]"}`,
	}
	for name, line := range bad {
		path := filepath.Join(t.TempDir(), "rows.jsonl")
		if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := readSamples(options{samples: path}); err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// CEP-V0-002/003: the committed fixture runs offline from its chunk corpus;
// file and line coverage/precision follow ContextBench's definitions, with a
// ranked file covering all of its lines, and the report pins the rows digest.
func TestContextBenchFixtureScoresFileAndLineCoverage(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	directory := filepath.Join("testdata", "contextbench")
	corvintGo := filepath.Join(t.TempDir(), "corvint")
	if err := os.WriteFile(corvintGo, []byte("#!/bin/sh\necho Corvint test\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	configuration := options{
		samples: filepath.Join(directory, "rows.jsonl"), corpus: filepath.Join(directory, "corpus"),
		corvintGo: corvintGo, limit: 5, taskTypes: map[string]bool{}, arms: map[string]bool{"context": true},
	}
	answers := map[string]arm{"negative count": {Ranked: []string{"ring.go", "README.md"}, State: "READY"}}
	report, err := bench(context.Background(), configuration, fakeCorvint(answers), fakeCorvint(answers), fakeImpact(nil), fakeAffected(nil))
	if err != nil {
		t.Fatal(err)
	}
	if digest := report["samples_sha256"]; digest != "9678b04832679f000e2eb5cac3230c5698f34b0f78c1c195cedfb0250af61fbc" {
		t.Fatalf("samples_sha256 = %v", digest)
	}
	want := map[string]map[string]float64{
		// Gold ring.go 3-8 (two spans merged) and ring_test.go 2-3: 8 lines.
		// Ranked ring.go (10 lines) and README.md (2 lines): 12 lines, 6 shared.
		"ringbuf-len-wrap": {"cb_file_coverage": 0.5, "cb_file_precision": 0.5, "cb_line_coverage": 0.75, "cb_line_precision": 0.5},
		// An empty answer covers nothing and is vacuously precise.
		"ringbuf-no-answer": {"cb_file_coverage": 0, "cb_file_precision": 1, "cb_line_coverage": 0, "cb_line_precision": 1},
	}
	details := report["details"].([]sampleReport)
	if len(details) != 2 {
		t.Fatalf("details = %d", len(details))
	}
	for _, detail := range details {
		got := detail.Metrics["context"]
		for key, value := range want[detail.ID] {
			if got[key] != value {
				t.Errorf("%s %s = %v, want %v (%v)", detail.ID, key, got[key], value, got)
			}
		}
	}
}
