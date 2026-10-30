package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// resealTrace applies change to a stored row and recomputes its identity over
// the typed canonical basis, as a schema-3 writer would.
func resealTrace(t testing.TB, raw []byte, change func(map[string]any)) []byte {
	t.Helper()
	value, err := decodeTrace(bytes.TrimSpace(raw))
	if err != nil {
		t.Fatal(err)
	}
	row := value.(map[string]any)
	change(row)
	preimage, err := canonicalTypedTrace(cloneWithout(row, "trace_id"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(preimage)
	row["trace_id"] = hex.EncodeToString(digest[:])
	sealed, err := canonicalTypedTrace(row)
	if err != nil {
		t.Fatal(err)
	}
	return append(sealed, '\n')
}

func asProducer(producer string) func(map[string]any) {
	return func(row map[string]any) {
		row["schema_version"] = json.Number("3")
		row["producer"] = producer
	}
}

// LTPM-V0-015 and LOD-V0-035: the standalone verifier admits schema-3 rows of
// either verification shape under the v2 registry and never discloses the
// producer; the actual dashboard producer agrees.
func TestProducerTraceConformance(t *testing.T) {
	root, revision := plantedRepository(t)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	command := resealTrace(t, plantedTraceRow(t, revision), asProducer("pi-tool"))
	typed := resealTrace(t, typedPlantedRow(t, revision, []any{"printf", "a b"}), asProducer("cli"))
	for _, row := range [][]byte{command, typed} {
		if _, err := validateTraceRow(bytes.TrimSpace(row), revision, acceptingTraceAuthority{}); err != nil {
			t.Fatalf("schema-3 row refused: %v %s", err, row)
		}
	}
	for name, change := range map[string]func(map[string]any){
		"stored UNKNOWN":    asProducer("UNKNOWN"),
		"unknown producer":  asProducer("agent"),
		"missing producer":  func(row map[string]any) { row["schema_version"] = json.Number("3") },
		"schema-2 producer": func(row map[string]any) { row["producer"] = "cli" },
	} {
		bad := resealTrace(t, typedPlantedRow(t, revision, []any{"printf", "a b"}), change)
		if _, err := validateTraceRow(bytes.TrimSpace(bad), revision, acceptingTraceAuthority{}); err == nil {
			t.Errorf("%s admitted", name)
		}
	}
	changed := bytes.Replace(command, []byte(`"producer":"pi-tool"`), []byte(`"producer":"dogfood"`), 1)
	if _, err := validateTraceRow(bytes.TrimSpace(changed), revision, acceptingTraceAuthority{}); err == nil {
		t.Error("producer outside the identity admitted")
	}

	mixed := append(append(plantedTraceRow(t, revision), command...), typed...)
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
	if len(corpus.sources) != 1 || corpus.sources[0].rows != 3 || !corpus.sources[0].hasV2 {
		t.Fatalf("schema-3 corpus: %+v", corpus)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("schema-3 snapshot: %v", err)
	}
	if bytes.Contains(expected, []byte(`"producer":`)) || bytes.Contains(expected, []byte("pi-tool")) {
		t.Fatal("snapshot disclosed the producer")
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
	if err := verifySnapshot(actual); err != nil {
		t.Fatalf("actual snapshot: %v", err)
	}
	assertTraceBlackBox(t, root, manifestRaw, actual)
}
