package console

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestSpecLookupRefusesEscapingFile guards the one console join that reads a
// requirement's source file: docs/specs/REQUIREMENTS.tsv is generated but
// parsed without validating its File field, and the console is the one
// surface that renders repository-derived paths to a browser, so a row whose
// File escapes the root must be refused through Clause.Err rather than
// opened.
func TestSpecLookupRefusesEscapingFile(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "docs", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tsv := "id\tfile\tline\ttitle\nREQ-1\t../../etc/passwd\t1\tEscape\n"
	if err := os.WriteFile(filepath.Join(specsDir, "REQUIREMENTS.tsv"), []byte(tsv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "INDEX.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	clause := SpecLookup{Root: root}.Resolve("REQ-1")
	if clause.Err == "" {
		t.Fatalf("expected a refusal for an escaping File field, got clause %+v", clause)
	}
	if clause.Text != "" {
		t.Fatalf("escaping file must not be read, got text %q", clause.Text)
	}
	if !strings.Contains(clause.Err, "escapes") {
		t.Fatalf("refusal should name the escape, got %q", clause.Err)
	}
}

// TestSpecLookupRefusesSymlinkedParent covers LAC-V0-018 for a symlink that is
// not the final component: a committed docs/specs link to a directory outside
// the worktree must not be followed to regular files there.
func TestSpecLookupRefusesSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	tsv := "id\tfile\tline\ttitle\nREQ-1\tdocs/specs/clause.md\t1\tOutside\n"
	for name, content := range map[string]string{"REQUIREMENTS.tsv": tsv, "INDEX.json": "[]", "clause.md": "- `REQ-1`: outside the worktree\n"} {
		if err := os.WriteFile(filepath.Join(outside, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs", "specs")); err != nil {
		t.Fatal(err)
	}
	clause := SpecLookup{Root: root}.Resolve("REQ-1")
	if clause.Err == "" || clause.Text != "" {
		t.Fatalf("a symlinked docs/specs parent was followed outside the worktree: text %q err %q", clause.Text, clause.Err)
	}
}

// TestSpecLookupReportsLineOutsideFile covers LAC-V0-008 for the clause join:
// a REQUIREMENTS.tsv row naming a line the spec file does not have is a gap,
// never an empty clause rendered as if it had been read.
func TestSpecLookupReportsLineOutsideFile(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "docs", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tsv := "id\tfile\tline\ttitle\nREQ-1\tdocs/specs/short.md\t40\tStale\n"
	for name, content := range map[string]string{"REQUIREMENTS.tsv": tsv, "INDEX.json": "[]", "short.md": "# Short\n- `REQ-1`: one line\n"} {
		if err := os.WriteFile(filepath.Join(specsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	clause := SpecLookup{Root: root}.Resolve("REQ-1")
	if clause.Err == "" || clause.Text != "" {
		t.Fatalf("want a gap for line 40 of a 3-line file, got text %q err %q", clause.Text, clause.Err)
	}
}

// TestReadBoundedRefusesSymlink is the F6 regression: a committed symlink to
// /dev/zero must be refused before it is ever opened, not read until the
// size limit trips. os.ReadFile following the link reached ~3.8 GiB of heap
// in under two seconds because /dev/zero reports size 0 and streams forever;
// readBounded's Lstat now sees the link itself and refuses in microseconds,
// well under the limit bytes, with no meaningful heap growth. A symlink to
// an ordinary file outside the worktree must be refused the same way.
func TestReadBoundedRefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-to-device semantics are unix-specific")
	}
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skipf("no /dev/zero on this host: %v", err)
	}

	t.Run("symlink to /dev/zero is refused, fast, without growing the heap", func(t *testing.T) {
		root := t.TempDir()
		link := filepath.Join(root, "devzero-link")
		if err := os.Symlink("/dev/zero", link); err != nil {
			t.Fatal(err)
		}

		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		start := time.Now()
		data, err := readBounded(root, "devzero-link", maxSpecIndexBytes)
		elapsed := time.Since(start)

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		if err == nil {
			t.Fatalf("expected a refusal, got %d bytes", len(data))
		}
		if !strings.Contains(err.Error(), "non-regular") {
			t.Fatalf("refusal should name the non-regular file, got %q", err)
		}
		// A hang detector, not a budget (decision 0082): the heap bound below is
		// the real assertion, and a loaded host must not fail this.
		if elapsed > 60*time.Second {
			t.Fatalf("refusal took %s, want it to return before ever reading", elapsed)
		}
		const heapBound = 16 << 20 // 16 MiB: comfortably above test overhead, far below the ~3.8 GiB regression.
		if after.HeapAlloc > before.HeapAlloc && after.HeapAlloc-before.HeapAlloc > heapBound {
			t.Fatalf("heap grew by %d bytes reading a refused symlink, want it bounded near zero", after.HeapAlloc-before.HeapAlloc)
		}
	})

	t.Run("symlink to a regular file outside the worktree is refused", func(t *testing.T) {
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(outside, []byte("outside the worktree"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "escape-link")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		if _, err := readBounded(root, "escape-link", maxSpecIndexBytes); err == nil {
			t.Fatal("expected a refusal for a symlink to a file outside the worktree")
		}
	})
}
