package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/evalrepo"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func runEvalProcess(t *testing.T, root, golden string) processResult {
	t.Helper()
	arguments := []string{"--root", root, "eval", "--goldens", golden}
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return execute(t, candidate)
}

func TestFreshProcessEvalMatchesPythonExceptLatency(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	golden := filepath.Join(root, "empty-eval.json")
	if err := os.WriteFile(golden, []byte("{\"schemaVersion\":1,\"cases\":[]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repositoryBytesDigest(t, root)
	candidate := runEvalProcess(t, root, golden)
	if candidate.exit != 0 {
		t.Fatalf("candidate exit=%d stderr=%s", candidate.exit, candidate.stderr)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("eval changed repository bytes")
	}
	normalized := normalizeEvalLatency(t, candidate.stdout)
	if !bytes.Contains(normalized, []byte(`"cases":[]`)) || !bytes.Contains(normalized, []byte(`"latency_ms":0`)) {
		t.Fatalf("unexpected normalized eval output: %s", normalized)
	}
}

func normalizeEvalLatency(t *testing.T, raw []byte) []byte {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode eval output: %v", err)
	}
	evaluation, ok := payload["evaluation"].(map[string]any)
	if !ok {
		t.Fatal("evaluation object missing")
	}
	metrics, ok := evaluation["metrics"].(map[string]any)
	if !ok {
		t.Fatal("metrics object missing")
	}
	latency, ok := metrics["latency_ms"].(float64)
	if !ok || latency < 0 {
		t.Fatalf("latency_ms=%v", metrics["latency_ms"])
	}
	metrics["latency_ms"] = float64(0)
	encoded, err := evalrepo.Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func TestEvalArgumentAndMissingCorpusErrorsMatchPython(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	for _, arguments := range [][]string{
		{"--root", root, "eval", "--goldens"},
		{"--root", root, "eval", "--unknown"},
		{"--root", root, "eval", "--goldens", "missing.json"},
	} {
		candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
		candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
		candidateResult := execute(t, candidate)
		if candidateResult.exit != 2 || len(candidateResult.stdout) != 0 || len(candidateResult.stderr) == 0 {
			t.Fatalf("arguments=%v result=%#v", arguments, candidateResult)
		}
	}
}

func TestEvalDoesNotReadStdin(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	golden := filepath.Join(root, "empty-eval.json")
	if err := os.WriteFile(golden, []byte("{\"schemaVersion\":1,\"cases\":[]}"), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := &forbiddenEvalReader{}
	var stdout, stderr bytes.Buffer
	exit := runContext(t.Context(), []string{"--root", root, "eval", "--goldens", golden}, reader, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 || stdout.Len() == 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	if reader.reads != 0 {
		t.Fatalf("stdin reads=%d", reader.reads)
	}
}

func TestEvalTraceFixtureScoresSecondArmAndReportsOrderedDeltas(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	parent := cemGit(t, root, "rev-parse", "HEAD^")
	if err := os.WriteFile(filepath.Join(root, "outcome.txt"), []byte("scored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "outcome.txt"}, {"commit", "-qm", "score fixture"}} {
		cemGit(t, root, arguments...)
	}
	head := cemGit(t, root, "rev-parse", "HEAD")
	golden := writeEvalGolden(t, root, "stable value implementation")
	fixture := writeEvalTraceFixture(t, root, head, parent, "repair stable value behavior")

	var stdout, stderr bytes.Buffer
	exit := runContext(t.Context(), []string{
		"--root", root, "eval", "--goldens", golden, "--trace-fixture", fixture,
	}, nil, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	evaluation := payload["evaluation"].(map[string]any)
	arm := evaluation["learned_trace_arm"].(map[string]any)
	metrics := arm["metrics"].(map[string]any)
	if metrics["passed_trace_count"] != float64(1) || metrics["trace_state"] != "ready" {
		t.Fatalf("learned trace metrics=%v", metrics)
	}
	deltas := evaluation["learned_trace_delta"].([]any)
	want := []string{
		"critical_evidence_misses", "abstention_accuracy", "epistemic_state_accuracy",
		"serialized_result_byte_weighted_precision", "recall", "top_five_task_success",
	}
	if len(deltas) != len(want) {
		t.Fatalf("delta count=%d want=%d", len(deltas), len(want))
	}
	for index, raw := range deltas {
		delta := raw.(map[string]any)
		if delta["metric"] != want[index] || delta["classification"] != "not distinguished" || delta["delta"] != "not distinguished" {
			t.Fatalf("delta[%d]=%v", index, delta)
		}
	}
}

func TestEvalTraceFixtureRejectsSharedTaskOrOutcomeCommit(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	parent := cemGit(t, root, "rev-parse", "HEAD^")
	head := cemGit(t, root, "rev-parse", "HEAD")
	const task = "stable value implementation"
	golden := writeEvalGolden(t, root, task)
	for _, test := range []struct {
		name, fixtureTask, fixtureRevision, message string
	}{
		{"task", "  " + task + "  ", parent, "fixture trace shares a scored task"},
		{"outcome commit", "repair stable value behavior", head, "fixture trace shares a scored outcome commit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := writeEvalTraceFixture(t, root, head, test.fixtureRevision, test.fixtureTask)
			var stdout, stderr bytes.Buffer
			exit := runContext(t.Context(), []string{
				"--root", root, "eval", "--goldens", golden, "--trace-fixture", fixture,
			}, nil, &stdout, &stderr)
			if exit != 2 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte(test.message)) {
				t.Fatalf("exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
			}
		})
	}
}

func TestEvalWithoutFixtureStillRefusesPassedLiveStore(t *testing.T) {
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
	if _, err := tracerecordrepo.Record(context.Background(), root, tracerecordrepo.Input{
		Task: "repair stable value behavior", ChangedPaths: []string{"pkg/main.go"}, Outcome: "passed",
	}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := runContext(t.Context(), []string{"--root", root, "eval", "--goldens", golden}, nil, &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("native Go eval trace replay is not implemented")) {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
	}
}

func writeEvalGolden(t *testing.T, _ string, task string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eval.json")
	payload := map[string]any{
		"schemaVersion": 1,
		"cases": []any{map[string]any{
			"id": "stable-value", "mode": "query", "text": task, "limit": 10,
			"expected": map[string]any{"relevant": []string{"learned-path:pkg/main.go"}},
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeEvalTraceFixture(t *testing.T, _ string, scoredRevision, outcomeRevision, task string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trace-fixture.json")
	record, err := trace.NewRecord(trace.Input{
		Revision: outcomeRevision, Task: task, ChangedPaths: []string{"pkg/main.go"}, Outcome: "passed",
	}, []string{"pkg/main.go"})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"schema_version": 1,
		"repositories": []any{map[string]any{
			"scored_revision": scoredRevision,
			"traces": []any{map[string]any{
				"schema_version": record.SchemaVersion, "trace_id": record.TraceID, "revision": record.Revision,
				"task": record.Task, "opened_paths": record.OpenedPaths, "changed_paths": record.ChangedPaths,
				"verification": record.Verification, "outcome": record.Outcome,
			}},
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type forbiddenEvalReader struct{ reads int }

func (reader *forbiddenEvalReader) Read([]byte) (int, error) {
	reader.reads++
	return 0, io.ErrUnexpectedEOF
}
