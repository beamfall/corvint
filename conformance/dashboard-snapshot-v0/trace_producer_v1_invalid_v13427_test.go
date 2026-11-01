package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestProducerTraceConformanceV1InvalidStore_V13427 closes the LOD-V0-960
// known gap (LOD-V0-970): on a v1 store whose only member file holds a
// malformed line, so no member is admitted, the actual producer and the
// standalone verifier agree on the source's byteCount and contentSha256
// (both null), epistemicClass (OBSERVED) and currency (VALIDATED_AT), and on
// every other field outside the two decision-0548 qualifications.
func TestProducerTraceConformanceV1InvalidStore_V13427(t *testing.T) {
	root, revision := plantedRepository(t)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	store := append(plantedTraceRow(t, revision), []byte("{not json}\n")...)
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", revision+".jsonl"), store, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := parseTraceManifest(manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")}))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.sources) != 1 || len(corpus.sources[0].members) != 0 || len(corpus.sources[0].terminal) == 0 {
		t.Fatalf("want one source with no admitted member: %+v", corpus.sources)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(t.TempDir(), "dashboard")
	if raw, err := exec.CommandContext(t.Context(), "go", "build", "-o", candidate, "../../cmd/corvint-dashboard-snapshot").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, raw)
	}
	actual, err := exec.CommandContext(t.Context(), candidate, "snapshot", "--root", root, "--conformance", "--generated-at", manifest.generatedAt.Format("2006-01-02T15:04:05.000000000Z")).Output()
	if err != nil {
		t.Fatalf("producer: %v", err)
	}
	want, err := decodeTrace(expected)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeTrace(actual)
	if err != nil {
		t.Fatal(err)
	}
	source := want.(map[string]any)["sources"].([]any)[0].(map[string]any)
	if source["validity"] != "INVALID" || source["byteCount"] != nil || source["contentSha256"] != nil || source["epistemicClass"] != "OBSERVED" || source["currency"] != "VALIDATED_AT" {
		t.Fatalf("verifier source = %v", source)
	}
	if qualified, _ := withoutDisputedV1Qualifications(got.(map[string]any)); qualified == 0 {
		t.Fatal("the decision-0548 authority dispute changed shape")
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
		t.Fatalf("invalid v1 store: producer differs outside the NOT_PRODUCED qualifications\nexpected %s\nactual   %s", wantBytes, gotBytes)
	}
}
