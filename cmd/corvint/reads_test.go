package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readsRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// URE-V0-007: `reads` is read-only and enable/disable are the only mutations.
func TestRunReadsDigestAndToggle(t *testing.T) {
	t.Parallel()
	root := readsRepository(t)
	if !isReadsInvocation([]string{"--root", root, "reads"}) || isReadsInvocation([]string{"--root", root, "observations"}) {
		t.Fatal("dispatch predicate misidentified the verb")
	}

	var stdout, stderr bytes.Buffer
	if status := runReads([]string{"--root", root, "reads"}, &stdout, &stderr); status != 0 {
		t.Fatalf("digest status %d stderr=%s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "UNPLANNED-READS enabled=false") {
		t.Fatalf("digest output: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf("the digest must not create state: %v", err)
	}

	stdout.Reset()
	if status := runReads([]string{"--root", root, "reads", "enable"}, &stdout, &stderr); status != 0 {
		t.Fatalf("enable status %d stderr=%s", status, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint", "unplanned-reads.enabled")); err != nil {
		t.Fatalf("enable did not create the marker: %v", err)
	}
	if status := runReads([]string{"--root", root, "reads", "disable"}, &stdout, &stderr); status != 0 {
		t.Fatalf("disable status %d stderr=%s", status, stderr.String())
	}

	stderr.Reset()
	if status := runReads([]string{"--root", root, "reads", "--limit", "0"}, &stdout, &stderr); status != 2 {
		t.Fatalf("invalid --limit status %d", status)
	}
	if !strings.Contains(stderr.String(), "invalid-arguments") {
		t.Fatalf("error emission: %q", stderr.String())
	}
}

// URE-V0-007: --limit belongs to the digest alone, so it is refused beside
// enable or disable in either order.
func TestRunReadsRefusesLimitBesideAToggleInEitherOrder(t *testing.T) {
	t.Parallel()
	root := readsRepository(t)
	for _, arguments := range [][]string{
		{"--root", root, "reads", "enable", "--limit", "5"},
		{"--root", root, "reads", "--limit", "5", "enable"},
		{"--root", root, "reads", "--limit=5", "disable"},
	} {
		var stdout, stderr bytes.Buffer
		if status := runReads(arguments, &stdout, &stderr); status != 2 || !strings.Contains(stderr.String(), "unrecognized arguments: --limit") {
			t.Errorf("%v: status %d stderr=%s", arguments, status, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf("a refused toggle must not create state: %v", err)
	}
}
