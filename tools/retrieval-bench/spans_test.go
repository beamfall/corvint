package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// CEP-V0-755: a span-mode context packet is scored against gold spans; rows
// keyed `path` are read; a file-mode packet adds no span metric.
func TestCEPV0755SpanModeScoresGoldSpans(t *testing.T) {
	packet := []byte(`{"state":"OK","results":[{"id":"a.go"},{"path":"b.go"}],"spans":{"rows":[{"path":"a.go","start_line":10,"end_line":19},{"path":"a.go","start_line":15,"end_line":24},{"path":"c.go","start_line":1,"end_line":5}]}}`)
	answer, err := parseContextPacket(packet)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(answer.Ranked, ",") != "a.go,b.go" || !answer.SpanMode || len(answer.Spans) != 3 {
		t.Fatalf("answer = %+v", answer)
	}
	item := sample{Gold: map[string]any{
		"gold_spans": []any{map[string]any{"path": "a.go", "start_line": float64(20), "end_line": float64(29)}},
		"lines":      []any{map[string]any{"file": "d.go", "start_line": 1, "end_line": 10}, map[string]any{"file": "a.go", "start_line": 0, "end_line": 3}},
	}}
	scores := metrics{}
	spanMetrics(scores, answer, item)
	// Gold: a.go 20-29 (10 lines) and d.go 1-10 (10 lines); the malformed
	// a.go 0-3 row is ignored. Spans: a.go 10-24 (15 lines) and c.go 1-5 (5
	// lines); a.go 20-24 is shared (5 lines).
	if scores["span_line_coverage"] != 0.25 || scores["span_line_precision"] != 0.25 || scores["span_lines"] != 20 {
		t.Fatalf("scores = %v", scores)
	}
	fileMode, err := parseContextPacket([]byte(`{"state":"OK","results":[{"id":"a.go"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	unchanged := metrics{}
	spanMetrics(unchanged, fileMode, item)
	if fileMode.SpanMode || len(unchanged) != 0 {
		t.Fatalf("file mode scored spans: %+v %v", fileMode, unchanged)
	}
	empty := metrics{}
	spanMetrics(empty, arm{SpanMode: true}, item)
	if empty["span_line_coverage"] != 0 || empty["span_line_precision"] != 0 || empty["span_lines"] != 0 {
		t.Fatalf("empty spans = %v", empty)
	}
}

// CEP-V0-756: --jobs is bounded, refuses order-dependent modes, keeps the
// report order, never runs two samples of one snapshot at once and returns
// the first error.
func TestCEPV0756JobsBoundedAndOrdered(t *testing.T) {
	for _, bad := range []options{{jobs: 0}, {jobs: maxJobs + 1}, {jobs: 2, snapshotLatency: true}, {jobs: 2, contextPackets: "x"}} {
		if validateJobs(bad) == nil {
			t.Fatalf("validateJobs(%+v) accepted", bad)
		}
	}
	if _, err := parseOptions([]string{"--samples", "s.jsonl", "--corpus", "c", "--jobs", "4"}); err != nil {
		t.Fatal(err)
	}
	if _, err := parseOptions([]string{"--samples", "s.jsonl", "--corpus", "c", "--jobs", "17"}); err == nil {
		t.Fatal("--jobs 17 accepted")
	}
	var samples []sample
	for i := range 12 {
		samples = append(samples, sample{ID: string(rune('a' + i)), Repo: "o/r" + string(rune('0'+i%4)), BaseCommit: "c"})
	}
	var lock sync.Mutex
	running, peak := map[string]int{}, 0
	total := 0
	judgeOne := func(ctx context.Context, space *workspaces, ordinal int, item sample) (sampleReport, error) {
		key := item.Repo + "@" + item.BaseCommit
		lock.Lock()
		running[key]++
		total++
		if running[key] > 1 {
			lock.Unlock()
			return sampleReport{}, errors.New("two samples of one snapshot at once")
		}
		peak = max(peak, total)
		lock.Unlock()
		time.Sleep(5 * time.Millisecond)
		lock.Lock()
		running[key]--
		total--
		lock.Unlock()
		return sampleReport{ID: item.ID}, nil
	}
	reports, err := judgeSamples(context.Background(), options{jobs: 3}, samples, judgeOne)
	if err != nil {
		t.Fatal(err)
	}
	for i, report := range reports {
		if report.ID != samples[i].ID {
			t.Fatalf("report %d = %q, want %q", i, report.ID, samples[i].ID)
		}
	}
	if peak > 3 || peak < 2 {
		t.Fatalf("peak concurrency %d, want 2..3", peak)
	}
	failing := func(ctx context.Context, space *workspaces, ordinal int, item sample) (sampleReport, error) {
		if ordinal == 5 {
			return sampleReport{}, errors.New("boom")
		}
		return sampleReport{ID: item.ID}, nil
	}
	if _, err := judgeSamples(context.Background(), options{jobs: 4}, samples, failing); err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}

// CEP-V0-757: the report's corvint identity names the binary's version line,
// its stated commit (or UNKNOWN) and its digest.
func TestCEPV0757IdentityRecordsCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake corvint is a POSIX shell script (decision 0559: native Windows is unsupported for 1.0)")
	}
	commit := strings.Repeat("ab", 20)
	for version, want := range map[string]string{
		"corvint 1.0.0-rc.3.dev (build 44, commit " + commit + ")": commit,
		"corvint dev": "UNKNOWN",
	} {
		fake := filepath.Join(t.TempDir(), "corvint")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\necho '"+version+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		identity, err := corvintIdentity(context.Background(), fake)
		if err != nil {
			t.Fatal(err)
		}
		if identity["version"] != version || identity["commit"] != want || len(identity["sha256"].(string)) != 64 {
			t.Fatalf("identity = %v", identity)
		}
	}
}
