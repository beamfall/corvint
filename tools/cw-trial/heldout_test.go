package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHeldout(t *testing.T, rows string, count int) (string, string) {
	t.Helper()
	dir := t.TempDir()
	tasks := filepath.Join(dir, "tasks.jsonl")
	if err := os.WriteFile(tasks, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(rows))
	frozen := `{"name":"heldout-test","partition":"held-out","task_count":` + itoa(count) + `,"tasks_sha256":"` + hex.EncodeToString(sum[:]) + `"}`
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(frozen), 0o644); err != nil {
		t.Fatal(err)
	}
	return tasks, manifestPath
}

func itoa(value int) string {
	return strings.TrimSpace(strings.Repeat(" ", 0) + string(rune('0'+value)))
}

const heldoutRows = `{"changed_file":"","gold":["src/a.go"],"gold_kind":"source-file","id":"trace2code:1","mode":"retrieval","query":{"command":"go test ./.","failure_excerpt":"FAIL a_test.go:3"},"repository":"o/r@abc","source":{"corpus_chunk_file":"c/o__r/abc.chunks.jsonl","corpus_sha256":"X","release":"v2_trace2code","sample_id":"1"},"task":"FAIL a_test.go:3"}
{"changed_file":"src/b.go","gold":["src/b_test.go"],"gold_kind":"test-file","id":"edit2ripple:2","mode":"change","query":{"anchor_diff":"@@ -1 +1 @@\n-x\n+y","anchor_file":"src/b.go","intent":"fix b"},"repository":"o/r@abc","source":{"corpus_chunk_file":"c/o__r/abc.chunks.jsonl","corpus_sha256":"X","release":"v2_edit2ripple","sample_id":"2"},"task":"fix b"}
`

func TestImportHeldoutMapsRowsOneToOne(t *testing.T) {
	tasks, manifestPath := writeHeldout(t, heldoutRows, 2)
	document, err := heldoutToManifest(tasks, manifestPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if document.Partition != "held-out" || !strings.Contains(document.Source, "heldout-test: tasks.jsonl (tasks_sha256 ") || len(document.Tasks) != 2 {
		t.Fatalf("manifest: %+v", document)
	}
	first, second := document.Tasks[0], document.Tasks[1]
	if first.Kind != "retrieval" || first.Repo != "o/r" || first.BaseCommit != "abc" || first.ChangedFile != "" || first.Gold["source-file"][0] != "src/a.go" {
		t.Fatalf("first: %+v", first)
	}
	if !strings.HasPrefix(first.Text, "A test run failed") || !strings.Contains(first.Text, "Command: go test ./.") || !strings.HasSuffix(first.Text, "kind source-file.") {
		t.Fatalf("first text: %q", first.Text)
	}
	if second.Kind != "change" || second.ChangedFile != "src/b.go" || second.Gold["test-file"][0] != "src/b_test.go" || !strings.Contains(second.Text, "Diff of the changed file:\n@@ -1 +1 @@") {
		t.Fatalf("second: %+v", second)
	}
	source, ok := first.Source.(map[string]any)
	if !ok || source["release"] != "v2_trace2code" || source["sample_id"] != "1" {
		t.Fatalf("source must be carried verbatim: %v", first.Source)
	}
	for _, item := range document.Tasks {
		if err := validateTask(item); err != nil {
			t.Fatalf("%s: produced task must validate: %v", item.ID, err)
		}
	}
}

func TestImportHeldoutRefusesWhatItCannotMap(t *testing.T) {
	cases := map[string]string{
		"repository without a commit": strings.Replace(heldoutRows, `"repository":"o/r@abc"`, `"repository":"o/r"`, 1),
		"change row without a file":   strings.Replace(heldoutRows, `"changed_file":"src/b.go"`, `"changed_file":""`, 1),
		"unknown release":             strings.Replace(heldoutRows, `"release":"v2_trace2code"`, `"release":"v2_other"`, 1),
		"gold kind answer":            strings.Replace(heldoutRows, `"gold_kind":"source-file"`, `"gold_kind":"answer"`, 1),
		"empty gold":                  strings.Replace(heldoutRows, `"gold":["src/a.go"]`, `"gold":[]`, 1),
	}
	for name, rows := range cases {
		tasks, manifestPath := writeHeldout(t, rows, 2)
		if _, err := heldoutToManifest(tasks, manifestPath, ""); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
	tasks, manifestPath := writeHeldout(t, heldoutRows, 2)
	if err := os.WriteFile(tasks, []byte(heldoutRows+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := heldoutToManifest(tasks, manifestPath, ""); err == nil || !strings.Contains(err.Error(), "tasks_sha256") {
		t.Fatalf("edited rows must fail the digest check, got %v", err)
	}
}

func TestImportHeldoutVerifiesTheChunkFileDigest(t *testing.T) {
	chunkRoot := t.TempDir()
	chunk := filepath.Join(chunkRoot, "c", "o__r", "abc.chunks.jsonl")
	if err := os.MkdirAll(filepath.Dir(chunk), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"kind":"file","path":"src/a.go","content":"package a\n"}` + "\n")
	if err := os.WriteFile(chunk, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	rows := strings.ReplaceAll(heldoutRows, `"corpus_sha256":"X"`, `"corpus_sha256":"`+hex.EncodeToString(sum[:])+`"`)
	tasks, manifestPath := writeHeldout(t, rows, 2)
	if _, err := heldoutToManifest(tasks, manifestPath, chunkRoot); err != nil {
		t.Fatalf("matching digest: %v", err)
	}
	tasks, manifestPath = writeHeldout(t, heldoutRows, 2)
	if _, err := heldoutToManifest(tasks, manifestPath, chunkRoot); err == nil || !strings.Contains(err.Error(), "corpus_sha256") {
		t.Fatalf("mismatched digest must be refused, got %v", err)
	}
}
