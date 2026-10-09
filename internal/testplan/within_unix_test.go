//go:build unix

package testplan

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestRunWithinConfinesReads: with Within set, every provider document and map is opened through
// the root at read time, so a symbolic link or a parent directory replaced to point outside the
// root, or a FIFO, is refused rather than read (TCN-V0-012).
func TestRunWithinConfinesReads(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.spec.ts": "a\n"})
	outside := t.TempDir()
	document := receipt("e2e", map[string]string{filepath.Join(root, "a.spec.ts"): digestOf("a\n")})
	for _, dir := range []string{outside, filepath.Join(root, "evidence")} {
		writeFiles(t, dir, map[string]string{"receipt.json": document})
	}
	if err := os.Symlink(filepath.Join(outside, "receipt.json"), filepath.Join(root, "leak.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("evidence/receipt.json", filepath.Join(root, "inside.json")); err != nil {
		t.Fatal(err)
	}
	// A checked directory later replaced by a link to the outside.
	if err := os.Symlink(outside, filepath.Join(root, "swapped")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	within, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer within.Close()
	in := inputJSON(t, variation("V1"))
	run := func(tests, maps []string) error {
		_, err := Run(context.Background(), Request{Root: root, Input: in, Tests: tests, Maps: maps, Within: within})
		return err
	}
	for _, name := range []string{"leak.json", "swapped/receipt.json", "pipe.json", "../receipt.json", "absent.json"} {
		if got := code(run([]string{name}, nil)); got != "invalid-test-validity-receipt" {
			t.Errorf("tests %s: %s", name, got)
		}
		if got := code(run(nil, []string{name})); got != "appmap-invalid-map" {
			t.Errorf("maps %s: %s", name, got)
		}
	}
	for _, name := range []string{"evidence/receipt.json", "inside.json"} {
		if err := run([]string{name}, nil); err != nil {
			t.Errorf("tests %s inside the root: %v", name, err)
		}
	}
}
