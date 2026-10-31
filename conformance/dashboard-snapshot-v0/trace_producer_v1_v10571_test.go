package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestProducerTraceConformanceV1Only_V10571 compares the actual
// cmd/corvint-dashboard-snapshot producer with the standalone verifier on a
// v1-only store (LOD-V0-960). The store holds a failed row and a passed row
// with non-ASCII text, so it exercises the outcome buckets and the v1
// identity of Unicode text. The two sides must agree on every field except
// the two disputed qualifications, which stay NOT_PRODUCED under decision
// 0548, and both disputed differences must still be present (V1-0571).
func TestProducerTraceConformanceV1Only_V10571(t *testing.T) {
	root, revision := plantedRepository(t)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	failed := mutateTraceRow(t, plantedTraceRow(t, revision), func(row map[string]any) { row["outcome"] = "failed" })
	unicode := asciiSealedV1Row(t, plantedTraceRow(t, revision), "t\u00e2che \u2713 \U0001F600 <&> \u00e9")
	for _, row := range [][]byte{failed, unicode} {
		if _, err := validateTraceRow(bytes.TrimSpace(row), revision, acceptingTraceAuthority{}); err != nil {
			t.Fatalf("v1 row refused: %v %s", err, row)
		}
	}
	store := append(append(append(plantedTraceRow(t, revision), failed...), '\n'), append(unicode, '\n')...)
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", revision+".jsonl"), store, 0o600); err != nil {
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
	if len(corpus.sources) != 1 || corpus.sources[0].rows != 3 || corpus.sources[0].hasV2 {
		t.Fatalf("v1 corpus: %+v", corpus)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(t.TempDir(), "dashboard")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", candidate, "../../cmd/corvint-dashboard-snapshot")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, raw)
	}
	actual, err := exec.CommandContext(t.Context(), candidate, "snapshot", "--root", root, "--conformance", "--generated-at", manifest.generatedAt.Format("2006-01-02T15:04:05.000000000Z")).Output()
	if err != nil {
		t.Fatalf("producer: %v", err)
	}
	if bytes.Equal(expected, actual) {
		t.Fatal("v1 producer now equals the standalone expectation: settle LOD-V0-960 and remove the NOT_PRODUCED carve-out")
	}
	want, err := decodeTrace(expected)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeTrace(actual)
	if err != nil {
		t.Fatal(err)
	}
	if qualified, zeroes := withoutDisputedV1Qualifications(got.(map[string]any)); qualified == 0 || zeroes == 0 {
		t.Fatalf("disputed differences changed shape: %d ADAPTER_QUALIFIED values, %d zero buckets", qualified, zeroes)
	}
	delete(want.(map[string]any), "snapshotSha256")
	wantBytes, err := canonical(want, false)
	if err != nil {
		t.Fatal(err)
	}
	gotBytes, err := canonical(got, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		t.Fatalf("v1 producer differs outside the NOT_PRODUCED qualifications\nexpected %s\nactual   %s", wantBytes, gotBytes)
	}
	t.Log("NOT_PRODUCED: v1 source/inventory authorityClass and zero outcome buckets (LOD-V0-960, decision 0548)")
}

// withoutDisputedV1Qualifications rewrites the actual snapshot to the
// standalone side of the two LOD-V0-960 disputes and drops its own hash. It
// counts the ADAPTER_QUALIFIED values and zero outcome buckets it removed.
func withoutDisputedV1Qualifications(snapshot map[string]any) (qualified, zeroes int) {
	delete(snapshot, "snapshotSha256")
	for _, section := range []string{"sources", "data"} {
		for _, item := range snapshot[section].([]any) {
			entry := item.(map[string]any)
			if entry["authorityClass"] == "ADAPTER_QUALIFIED" {
				entry["authorityClass"] = "ADVISORY"
				qualified++
			}
			dimensions, _ := entry["dimensions"].([]any)
			for _, dimension := range dimensions {
				pair := dimension.(map[string]any)
				if pair["name"] == "authorityClass" && pair["value"] == "ADAPTER_QUALIFIED" {
					pair["value"] = "ADVISORY"
				}
			}
		}
	}
	usage := snapshot["usage"].([]any)
	kept := usage[:0]
	for _, item := range usage {
		metric := item.(map[string]any)
		if metric["name"] == "usage.trace.retained" && metric["value"] == "0" {
			zeroes++
			continue
		}
		kept = append(kept, metric)
	}
	snapshot["usage"] = kept
	return qualified, zeroes
}

// asciiSealedV1Row rewrites a v1 row's task and derives trace_id over the
// production writer's ASCII JSON (internal/trace.Encode), as a real v1 row
// with non-ASCII text carries it.
func asciiSealedV1Row(t testing.TB, raw []byte, task string) []byte {
	t.Helper()
	value, err := decodeTrace(bytes.TrimSpace(raw))
	if err != nil {
		t.Fatal(err)
	}
	trace := value.(map[string]any)
	trace["task"] = task
	basis, err := canonicalTypedTrace(cloneWithout(trace, "trace_id"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(basis)
	trace["trace_id"] = hex.EncodeToString(digest[:])
	row, err := canonicalTypedTrace(trace)
	if err != nil {
		t.Fatal(err)
	}
	return row
}
