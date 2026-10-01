package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func typedPlantedRow(t testing.TB, revision string, argv []any) []byte {
	t.Helper()

	value, err := decodeTrace(plantedTraceRow(t, revision))
	if err != nil {
		t.Fatal(err)
	}
	row := value.(map[string]any)
	row["schema_version"] = json.Number("2")
	row["verification"] = []any{map[string]any{"argv": argv, "kind": "argv"}}
	preimage, err := canonicalTypedTrace(cloneWithout(row, "trace_id"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(preimage)
	row["trace_id"] = hex.EncodeToString(digest[:])
	raw, err := canonicalTypedTrace(row)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// LOD-V0-035: the standalone verifier independently reads typed rows and
// derives one mixed physical member; no production trace/model package is used.
func TestTypedTraceConformance(t *testing.T) {
	root, revision := plantedRepository(t)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	row := typedPlantedRow(t, revision, []any{"printf", "a b", "", "{\"key\":1}", "|", "é"})
	if _, err := validateTraceRow(bytes.TrimSpace(row), revision, acceptingTraceAuthority{}); err != nil {
		t.Fatal(err)
	}
	for _, argv := range [][]any{{"x", nil}, {"x", "--password", "word"}, {"x", "\u0085"}, {"x", strings.Repeat("é", 513)}, {"x", strings.Repeat("😀", 512)}} {
		bad := typedPlantedRow(t, revision, argv)
		if _, err := validateTraceRow(bytes.TrimSpace(bad), revision, acceptingTraceAuthority{}); err == nil {
			t.Fatalf("admitted %#v", argv)
		}
	}
	bad := bytes.Replace(row, []byte(`"kind":"argv"`), []byte(`"kind":"argv","kind":"argv"`), 1)
	if _, err := validateTraceRow(bytes.TrimSpace(bad), revision, acceptingTraceAuthority{}); err == nil {
		t.Fatal("duplicate typed member admitted")
	}
	mixed := append(plantedTraceRow(t, revision), row...)
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", revision+".jsonl"), mixed, 0600); err != nil {
		t.Fatal(err)
	}
	manifestRaw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.sources) != 1 || len(corpus.sources[0].members) != 1 || corpus.sources[0].rows != 2 || !corpus.sources[0].hasV2 {
		t.Fatalf("mixed corpus: %+v", corpus)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("typed snapshot: %v", err)
	}
	assertTraceBlackBox(t, root, manifestRaw, expected)
	// Actual producer output must also pass the standalone verifier.
	candidate := filepath.Join(t.TempDir(), "dashboard")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", candidate, "../../cmd/corvint-dashboard-snapshot")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, raw)
	}
	command := exec.CommandContext(t.Context(), candidate, "snapshot", "--root", root, "--conformance", "--generated-at", manifest.generatedAt.Format("2006-01-02T15:04:05.000000000Z"))
	actual, err := command.Output()
	if err != nil {
		t.Fatalf("producer: %v %s", err, err.(*exec.ExitError).Stderr)
	}
	if err := verifySnapshot(actual); err != nil {
		t.Fatalf("actual snapshot: %v", err)
	}
	assertTraceBlackBox(t, root, manifestRaw, actual)
}

func TestTypedTraceTaskProfileIsNotV1(t *testing.T) {
	revision := strings.Repeat("a", 40)
	for _, item := range []struct {
		task  string
		valid bool
	}{{"https://user?x:pass@host", true}, {"x\u001c", false}} {
		raw := typedPlantedRow(t, revision, []any{"printf", "a b"})
		value, err := decodeTrace(raw)
		if err != nil {
			t.Fatal(err)
		}
		row := value.(map[string]any)
		row["task"] = item.task
		preimage, err := canonicalTypedTrace(cloneWithout(row, "trace_id"))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(preimage)
		row["trace_id"] = hex.EncodeToString(hash[:])
		raw, err = canonicalTypedTrace(row)
		if err != nil {
			t.Fatal(err)
		}
		_, err = validateTraceRow(raw, revision, acceptingTraceAuthority{})
		if (err == nil) != item.valid {
			t.Fatalf("task %q valid=%v err=%v", item.task, item.valid, err)
		}
	}
}
