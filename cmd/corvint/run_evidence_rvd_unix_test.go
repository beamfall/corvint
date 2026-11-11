// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runevidence"
)

// rvdStore is the fixture repository's run evidence store.
func rvdStore(root string) runevidence.Store {
	return runevidence.Store{Dir: runevidence.Locate(filepath.Join(root, ".git"))}
}

// RVD-V1-004, RVD-V1-001: one `corvint test` run appends exactly one record, the receipt's own
// bytes under their SHA-256; a second run adds a second record and leaves the first unchanged; each
// shard of a sharded run appends its own receipt; a dry run appends nothing.
func TestRVDV1004CLITestAppendsOneRecordPerRun(t *testing.T) {
	root, base, _ := sessionFixture(t, "pass")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run"); code != 0 {
		t.Fatalf("dry run %d %s", code, stderr)
	}
	if _, err := os.Lstat(rvdStore(root).Dir); !os.IsNotExist(err) {
		t.Fatal("a dry run wrote the run evidence store")
	}
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 || strings.Contains(stderr, runevidence.ReasonWriteFailed) {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	receipt, err := os.ReadFile(filepath.Join(root, ".git", "corvint", "test-run.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := rvdStore(root).Read()
	if err != nil || len(snapshot.Records) != 1 || !bytes.Equal(snapshot.Records[0].Data, receipt) ||
		snapshot.Records[0].Entry.ID != runevidence.ID(receipt) || len(snapshot.Records[0].Rows) != 1 ||
		snapshot.Records[0].Rows[0].TestID != "go:example.test/m/a" || snapshot.Records[0].Rows[0].Outcome != "passed" {
		t.Fatalf("store after one run: %+v %v", snapshot, err)
	}
	first := snapshot.Records[0].Entry.ID
	if err := os.WriteFile(filepath.Join(root, "a", "a.go"), []byte("package a\n\nfunc A() int { return 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change a again")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base); code != 0 {
		t.Fatalf("second run %d %s", code, stderr)
	}
	snapshot, err = rvdStore(root).Read()
	if err != nil || len(snapshot.Records) != 2 || snapshot.Records[0].Entry.ID != first || snapshot.Records[0].Entry.Tree == snapshot.Records[1].Entry.Tree {
		t.Fatalf("store after two runs: %+v %v", snapshot, err)
	}
	if data, err := os.ReadFile(filepath.Join(rvdStore(root).Dir, "records", first+".json")); err != nil || runevidence.ID(data) != first {
		t.Fatalf("the first record changed: %v", err)
	}
	for _, shard := range []string{"0/2", "1/2"} {
		if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--shard", shard); code != 0 {
			t.Fatalf("shard %s: %d %s", shard, code, stderr)
		}
	}
	if snapshot, err = rvdStore(root).Read(); err != nil || len(snapshot.Records) != 4 {
		t.Fatalf("two shards appended %d records, want 2 more: %v", len(snapshot.Records)-2, err)
	}
	for index, shard := range []string{"0", "1"} {
		receipt, err := os.ReadFile(filepath.Join(root, ".git", "corvint", "test-run."+shard+".json"))
		if err != nil || !bytes.Equal(snapshot.Records[2+index].Data, receipt) {
			t.Fatalf("shard %s record is not its receipt: %v", shard, err)
		}
	}
	if status := affectedGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("a run changed the worktree: %q", status)
	}
}

// RVD-V1-010: when the store cannot be written the run prints run-evidence-write-failed with the
// cause, still writes its receipt and keeps its exit code.
func TestRVDV1010CLIWriteFailureKeepsExitCode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root, base, _ := sessionFixture(t, "pass")
	store := rvdStore(root).Dir
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(store, 0o700) })
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 || !strings.Contains(stderr, "corvint-test: "+runevidence.ReasonWriteFailed+" ") ||
		!strings.Contains(stderr, "corvint-test: PASS selected units passed: 1 of 1") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "corvint", "test-run.json")); err != nil {
		t.Fatalf("the receipt was not written: %v", err)
	}
	// A linked store directory is refused, never followed.
	if err := os.Chmod(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(store); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, store); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 || !strings.Contains(stderr, runevidence.ReasonWriteFailed) {
		t.Fatalf("linked store: code %d stderr %q", code, stderr)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("the linked store received %v", entries)
	}
}
