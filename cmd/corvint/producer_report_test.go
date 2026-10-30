package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// LTPM-V0-016: calibrate counts every record read by producer and can leave a
// producer out of its sample without writing anything.
func TestCalibrateCountsAndExcludesProducers_LTPM016(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 2)
	if _, err := tracerecordrepo.Record(context.Background(), root, tracerecordrepo.Input{Producer: trace.ProducerPiTool,
		Task: "model reported outcome", ChangedPaths: []string{"cache/reader.go"}, Outcome: "passed",
	}); err != nil {
		t.Fatal(err)
	}
	before := calibrateTreeDigest(t, root)
	code, stdout, stderr := runCalibrateForTest(t, "--root", root, "calibrate", "--exclude-producer", "pi-tool", "--exclude-producer=pi-tool")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var payload struct {
		SampleSize int            `json:"sample_size"`
		Producers  map[string]int `json:"producers"`
		Excluded   []string       `json:"excluded_producers"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"cli": 2, "dogfood": 0, "pi-tool": 1, "UNKNOWN": 0}
	if payload.SampleSize != 2 || !reflect.DeepEqual(payload.Producers, want) || !reflect.DeepEqual(payload.Excluded, []string{"pi-tool"}) {
		t.Fatalf("payload %s", stdout)
	}
	if after := calibrateTreeDigest(t, root); after != before {
		t.Fatal("calibrate --exclude-producer changed repository or trace state")
	}
	code, _, _ = runCalibrateForTest(t, "--root", root, "calibrate")
	if code != 0 {
		t.Fatal("unfiltered calibrate failed")
	}
	if code, _, stderr := runCalibrateForTest(t, "--root", root, "calibrate", "--exclude-producer", "agent"); code != 2 || !bytes.Contains([]byte(stderr), []byte("--exclude-producer")) {
		t.Fatalf("admitted unknown producer: code=%d stderr=%s", code, stderr)
	}
}

// LTPM-V0-016: eval discloses the stored traces by producer, and excluding a
// producer changes only what this read admits, never the store.
func TestEvalCountsAndExcludesProducers_LTPM016(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".context-corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", ".gitignore"}, {"commit", "-qm", "ignore local traces"}} {
		cemGit(t, root, arguments...)
	}
	golden := filepath.Join(t.TempDir(), "empty-eval.json")
	if err := os.WriteFile(golden, []byte("{\"schemaVersion\":1,\"cases\":[]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := runContext(t.Context(), []string{"--root", root, "eval", "--goldens", golden}, nil, &stdout, &stderr); exit != 0 || bytes.Contains(stdout.Bytes(), []byte("producers")) {
		t.Fatalf("empty store changed baseline bytes: exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
	}
	for _, input := range []tracerecordrepo.Input{
		{Producer: trace.ProducerPiTool, Task: "model reported success", ChangedPaths: []string{"pkg/main.go"}, Outcome: "passed"},
		{Producer: trace.ProducerCLI, Task: "operator recorded failure", ChangedPaths: []string{"pkg/main.go"}, Outcome: "failed"},
	} {
		if _, err := tracerecordrepo.Record(context.Background(), root, input); err != nil {
			t.Fatal(err)
		}
	}
	before := calibrateTreeDigest(t, root)
	stdout.Reset()
	stderr.Reset()
	if exit := runContext(t.Context(), []string{"--root", root, "eval", "--goldens", golden}, nil, &stdout, &stderr); exit != 2 {
		t.Fatalf("passed pi-tool trace was not refused: exit=%d stderr=%s", exit, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := runContext(t.Context(), []string{"--root", root, "eval", "--goldens", golden, "--exclude-producer", "pi-tool"}, nil, &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, &stderr)
	}
	var payload struct {
		Evaluation struct {
			Producers map[string]int `json:"trace_producers"`
			Excluded  []string       `json:"excluded_producers"`
		} `json:"evaluation"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"cli": 1, "dogfood": 0, "pi-tool": 1, "UNKNOWN": 0}
	if !reflect.DeepEqual(payload.Evaluation.Producers, want) || !reflect.DeepEqual(payload.Evaluation.Excluded, []string{"pi-tool"}) {
		t.Fatalf("payload %s", &stdout)
	}
	if after := calibrateTreeDigest(t, root); after != before {
		t.Fatal("eval --exclude-producer changed repository or trace state")
	}
	stderr.Reset()
	if exit := runContext(t.Context(), []string{"--root", root, "eval", "--learn-slot-weights", "--goldens", golden, "--exclude-producer", "cli"}, nil, &stdout, &stderr); exit != 2 || !bytes.Contains(stderr.Bytes(), []byte("--exclude-producer")) {
		t.Fatalf("slot-weight learning admitted --exclude-producer: %s", &stderr)
	}
}
