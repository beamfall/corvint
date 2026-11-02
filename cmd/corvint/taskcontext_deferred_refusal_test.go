package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestCompileTaskContextRefusesAfterInstructionReads_IDXSNAPV0814: an
// AGENTS.override.md that only the instruction load set reads, overwritten in
// place under a deferred gob load, makes compileTaskContext return
// ErrSnapshotRefused instead of a packet that lists it unavailable, and
// runTaskContext's eager reload then emits the packet (IDX-SNAP-V0-814).
func TestCompileTaskContextRefusesAfterInstructionReads_IDXSNAPV0814(t *testing.T) {
	t.Setenv("CORVINT_INDEX_STORE", t.TempDir())
	root := t.TempDir()
	override := "override rules that only the instruction load set reads\n"
	for name, body := range map[string]string{
		"go.mod":             "module example.test/refusal\n\ngo 1.27.0\n",
		"AGENTS.override.md": override,
		"app/source.go":      "package app\n\nfunc Source() {}\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.test", "commit", "-qm", "fixture"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := contextindex.WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	options := taskContextOptions{
		root: root, task: "where is Source", subject: "app/source.go", limit: 5, lsp: "off", lspSet: true,
		instructionHost: "codex", instructionVersion: "0.153.2", instructionCWD: "app", instructionProfile: "default",
		instructionFlags: map[string]bool{"--instruction-load-set": true},
	}
	if _, _, _, err := compileTaskContext(context.Background(), options, loadContextSnapshot); err != nil {
		t.Fatalf("an intact snapshot: %v", err)
	}
	data, err := os.ReadFile(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(data, []byte(override)) != 1 {
		t.Fatal("the override body is not in the snapshot exactly once")
	}
	forged := bytes.Replace(data, []byte(override), bytes.ToUpper([]byte(override)), 1)
	if err := os.WriteFile(receipt.Path, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := compileTaskContext(context.Background(), options, loadContextSnapshot); !errors.Is(err, contextindex.ErrSnapshotRefused) {
		t.Fatalf("a packet was emitted over a refused instruction body: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := runTaskContext(context.Background(), options, &stdout, &stderr); code != 0 || !bytes.Contains(stdout.Bytes(), []byte("instruction_load_set")) {
		t.Fatalf("the eager reload did not emit the packet: exit %d\n%s", code, stderr.String())
	}
}
