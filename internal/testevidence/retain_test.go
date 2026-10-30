package testevidence

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestRetainReportsDocumentItsPruneRemoved: with 32 own documents named after the clock, the
// new document is the one pruned, so Retain fails instead of naming a file that is gone
// (LPCV-V0-055), and the directory still holds the newest 32 names.
func TestRetainReportsDocumentItsPruneRemoved(t *testing.T) {
	root := t.TempDir()
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := range MaxRetained {
		name := fmt.Sprintf("corvint-js-test-provider-%019d-%016x.json", int64(9_000_000_000_000_000_000)+int64(index), index)
		if err := os.WriteFile(filepath.Join(evidence, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	name, err := Retain(root, "corvint-js-test-provider", []byte("{}"))
	if err == nil {
		t.Fatalf("retain of a pruned document %q succeeded", name)
	}
	remaining, _ := filepath.Glob(filepath.Join(evidence, "corvint-js-test-provider-*.json"))
	if len(remaining) != MaxRetained {
		t.Fatalf("remaining=%d", len(remaining))
	}
}

// TestRetainPrunesCrashLeftoverTemporaries: 225 temporaries a crash left before rename, beside 32
// documents, made the directory hold 257 entries, which discovery refuses. Prune now removes the
// producer's temporaries named below its oldest kept document, and keeps a newer temporary and
// another producer's (LPCV-V0-055).
func TestRetainPrunesCrashLeftoverTemporaries(t *testing.T) {
	root := t.TempDir()
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	names := []string{".other-0000000000000000001-0000000000000000.tmp"}
	for index := range 225 {
		names = append(names, fmt.Sprintf(".p-%019d-%016x.tmp", 1000+index, index))
	}
	for index := range MaxRetained {
		names = append(names, fmt.Sprintf("p-%019d-%016x.json", 5000+index, index))
	}
	kept := fmt.Sprintf(".p-%019d-%016x.tmp", 6000, 0)
	for _, name := range append(names, kept) {
		if err := os.WriteFile(filepath.Join(evidence, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Retain(root, "p", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	temporaries, _ := filepath.Glob(filepath.Join(evidence, ".*.tmp"))
	if len(temporaries) != 2 || filepath.Base(temporaries[0]) != names[0] || filepath.Base(temporaries[1]) != kept {
		t.Fatalf("temporaries=%v", temporaries)
	}
}
