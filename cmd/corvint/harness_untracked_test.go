package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAHI022HarnessFileChangeCodesUntrackedPath pins V1-0450: a file-change
// for a newly created, untracked path refuses with the structured
// untracked-impact-path code an adapter can treat as expected, while the
// message bytes and the uncoded `impact` refusal stay as they were.
func TestAHI022HarnessFileChangeCodesUntrackedPath(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	if err := os.WriteFile(filepath.Join(root, "cache", "created.go"), []byte("package cache\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const prefix = "impact paths are not tracked at revision "
	var stdout, stderr bytes.Buffer
	exit := runContext(context.Background(), cliArguments(root, "file-change"), strings.NewReader(`{"paths":["cache/created.go"]}`), &stdout, &stderr)
	var envelope map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &envelope); exit != 2 || stdout.Len() != 0 || err != nil {
		t.Fatalf("harness exit=%d stdout=%q stderr=%q err=%v", exit, &stdout, &stderr, err)
	}
	message, _ := envelope["error"].(string)
	if envelope["code"] != "untracked-impact-path" || envelope["ok"] != false || len(envelope) != 3 || !strings.HasPrefix(message, prefix) || !strings.Contains(message, "cache/created.go") {
		t.Fatalf("harness envelope = %#v", envelope)
	}
	stdout.Reset()
	stderr.Reset()
	exit = runContext(context.Background(), []string{"--root", root, "impact", "cache/created.go"}, strings.NewReader(""), &stdout, &stderr)
	var impact map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &impact); exit != 2 || err != nil {
		t.Fatalf("impact exit=%d stderr=%q err=%v", exit, &stderr, err)
	}
	if _, coded := impact["code"]; coded || impact["error"] != message {
		t.Fatalf("impact refusal changed: %#v, harness message %q", impact, message)
	}
	// The code is an expected state, not an unsupported capability, so no
	// SOL-V0-007 row is appended for it.
	ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if bytes.Contains(ledger, []byte("untracked-impact-path")) {
		t.Fatalf("ledger recorded the untracked refusal: %s", ledger)
	}
}
